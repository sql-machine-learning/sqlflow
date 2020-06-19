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
	"bytes"
	"fmt"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"sqlflow.org/sqlflow/pkg/verifier"
)

// Create prediction table using the `PredictStmt`.
func createPredictionResultTable(predStmt *ir.PredictStmt, db *database.DB, session *pb.Session) error {
	dropStmt := fmt.Sprintf("drop table if exists %s;", predStmt.ResultTable)
	if _, e := db.Exec(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}
	flds, fts, e := getSQLFieldType(predStmt.Select, db)
	if e != nil {
		return e
	}

	var b bytes.Buffer
	// NOTE(typhoonzero): predStmt.TrainStmt may be nil, because the model may not loaded when
	// creating prediction table.
	trainLabelColumn := ""
	if predStmt.TrainStmt != nil {
		trainLabelColumn = predStmt.TrainStmt.Label.GetFieldDesc()[0].Name
	}
	resultColumnName := predStmt.ResultColumn
	resultColumnType := ""
	fmt.Fprintf(&b, "create table %s (", predStmt.ResultTable)
	for idx, colType := range fts {
		stype, e := fieldType(db.DriverName, colType)
		if e != nil {
			return e
		}
		fldName := flds[idx]
		// When predicting use validation table, we should find the label column type
		// using the label column name from train table.
		if fldName == trainLabelColumn {
			resultColumnType = stype
			if resultColumnName == trainLabelColumn {
				continue
			}
		}
		// result column have the same name, do not add as feature column
		if fldName == resultColumnName {
			resultColumnType = stype
			continue
		}
		fmt.Fprintf(&b, "%s %s, ", fldName, stype)
	}

	// TODO(Yancey1989): For the current implementation, the prediction result column
	// type is derivated by the pred-select-statement, the better way is derivating
	// the result column type by the prediction result.
	//
	// label column not found in predict table, create a column specified by PREDICT clause:
	if resultColumnType == "" {
		// NOTE(typhoonzero): Clustering model may not have label in select statement, default use INT type
		resultColumnType = "INT"
	}
	// mapping to DBMS column type from the result column type
	stype, e := fieldType(db.DriverName, resultColumnType)
	if e != nil {
		return fmt.Errorf("mapping to DBMS column type failed, %s", e)
	}
	if db.DriverName == "hive" {
		fmt.Fprintf(&b, "%s %s) ROW FORMAT DELIMITED FIELDS TERMINATED BY \"\\001\" STORED AS TEXTFILE;", resultColumnName, stype)
	} else {
		fmt.Fprintf(&b, "%s %s);", resultColumnName, stype)
	}

	createStmt := b.String()
	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}

// getSQLFieldType is quiet like verify but accept a SQL string as input, and returns
// an ordered list of the field types.
func getSQLFieldType(slct string, db *database.DB) ([]string, []string, error) {
	rows, err := verifier.FetchSamples(db, slct)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil, fmt.Errorf("query %s gives 0 row", slct)
	}

	if rows.Err() != nil {
		return nil, nil, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, err
	}

	ft := []string{}
	flds := []string{}
	for _, ct := range columnTypes {
		_, fld := verifier.Decomp(ct.Name())
		typeName := ct.DatabaseTypeName()
		flds = append(flds, fld)
		ft = append(ft, typeName)
	}

	return flds, ft, nil
}
