// Copyright 2020 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package executor

import (
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	"sqlflow.org/sqlflow/go/model"
	"sqlflow.org/sqlflow/go/verifier"
)

func getPredictionTableFieldNamesAndTypes(predStmt *ir.PredictStmt, db *database.DB) ([]string, []string, error) {
	slctFlds, slctTypes, e := verifier.GetSQLFieldType(predStmt.Select, db)
	if e != nil {
		return nil, nil, e
	}

	// NOTE(typhoonzero): predStmt.TrainStmt may be nil, because the model may not loaded when
	// creating prediction table.
	trainLabelColumn := ""
	if predStmt.TrainStmt != nil {
		trainLabelColumn = predStmt.TrainStmt.Label.GetFieldDesc()[0].Name
	}

	resultColumnName := predStmt.ResultColumn
	resultColumnType := ""
	var fieldNames []string
	var fieldTypes []string
	for idx, colType := range slctTypes {
		stype, e := fieldType(db.DriverName, colType)
		if e != nil {
			return nil, nil, e
		}
		fldName := slctFlds[idx]
		// When predicting use validation table, we should find the label column type
		// using the label column name from train table.
		// Skip label columns, and use predStmt.ResultColumn as the result column.
		if fldName == trainLabelColumn || fldName == resultColumnName {
			resultColumnType = stype
			continue
		}
		fieldNames = append(fieldNames, fldName)
		fieldTypes = append(fieldTypes, stype)
	}

	// TODO(Yancey1989): For the current implementation, the prediction result column
	// type is derivated by the pred-select-statement, the better way is derivating
	// the result column type by the prediction result.
	//
	// label column not found in predict table, create a column specified by PREDICT clause:
	strFieldType, e := stringFieldType(db.DriverName)
	if e != nil {
		return nil, nil, e
	}

	if resultColumnType == "" {
		// NOTE(typhoonzero): Clustering model may not have label in select statement, default use STRING type
		resultColumnType = strFieldType
	} else {
		// mapping to DBMS column type from the result column type
		resultColumnType, e = fieldType(db.DriverName, resultColumnType)
		if e != nil {
			return nil, nil, fmt.Errorf("mapping to DBMS column type failed, %s", e)
		}
	}
	fieldNames = append(fieldNames, resultColumnName)
	fieldTypes = append(fieldTypes, resultColumnType)

	extraFieldsInterface, ok := predStmt.Attributes["predict.extra_outputs"]
	if ok {
		extraFields, ok := extraFieldsInterface.(string)
		if !ok {
			return nil, nil, fmt.Errorf("predict.extra_outputs must be string")
		}
		extraFieldList := strings.Split(extraFields, ",")
		for _, field := range extraFieldList {
			field = strings.TrimSpace(field)
			if len(field) > 0 {
				fieldNames = append(fieldNames, field)
				fieldTypes = append(fieldTypes, strFieldType)
			}
		}
	}
	return fieldNames, fieldTypes, nil
}

// Create prediction table using the `PredictStmt`.
func createPredictionResultTable(predStmt *ir.PredictStmt, db *database.DB) error {
	dropStmt := fmt.Sprintf("drop table if exists %s;", predStmt.ResultTable)
	if _, e := db.Exec(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}

	names, types, e := getPredictionTableFieldNamesAndTypes(predStmt, db)
	if e != nil {
		return e
	}

	var fieldItems []string
	for idx := range names {
		fieldItems = append(fieldItems, fmt.Sprintf("%s %s", names[idx], types[idx]))
	}

	var template string
	if db.DriverName == "hive" {
		template = "CREATE TABLE %s (%s) ROW FORMAT DELIMITED FIELDS TERMINATED BY \"\\001\" STORED AS TEXTFILE;"
	} else {
		template = "CREATE TABLE %s (%s);"
	}

	createStmt := fmt.Sprintf(template, predStmt.ResultTable, strings.Join(fieldItems, ","))
	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}

func createExplainResultTable(db *database.DB, ir *ir.ExplainStmt, tableName string, modelType int, estimator string) error {
	dropStmt := fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, tableName)
	var e error
	if _, e = db.Exec(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}
	createStmt := ""
	// TODO(typhoonzero): Create XGBoostExplainer result table should be
	// moved to Python runtime shortly.
	if ir.Explainer == "XGBoostExplainer" {
		// User specified using XGBoost functions to get fscore, gain.
		// Create table with columns fscore, gain. Then each row records
		// a feature's fscore and gain value.
		columnDef := ""
		if db.DriverName == "mysql" {
			columnDef = "(feature VARCHAR(255), fscore FLOAT, gain FLOAT)"
		} else {
			columnDef = "(feature STRING, fscore STRING, gain STRING)"
		}
		createStmt = fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s %s", tableName, columnDef)
	} else if modelType == model.TENSORFLOW {
		if strings.HasPrefix(estimator, "BoostedTrees") {
			columnDef := ""
			if db.DriverName == "mysql" {
				columnDef = "(feature VARCHAR(255), dfc FLOAT, gain FLOAT)"
			} else {
				// Hive & MaxCompute
				columnDef = "(feature STRING, dfc STRING, gain STRING)"
			}
			createStmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s %s;`, tableName, columnDef)
		} else {
			labelCol, ok := ir.Attributes["label_col"]
			if !ok {
				return fmt.Errorf("need to specify WITH label_col=lable_col_name when explaining deep models")
			}
			createStmt, e = getCreateShapResultSQL(db, tableName, ir.Select, labelCol.(string))
			if e != nil {
				return e
			}
		}
	} else if modelType == model.XGBOOST {
		labelCol, ok := ir.Attributes["label_col"]
		if !ok {
			return fmt.Errorf("need to specify WITH label_col=lable_col_name when explaining xgboost models")
		}
		createStmt, e = getCreateShapResultSQL(db, tableName, ir.Select, labelCol.(string))
		if e != nil {
			return e
		}
	} else {
		return fmt.Errorf("not supported modelType %d for creating Explain result table", modelType)
	}

	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}
