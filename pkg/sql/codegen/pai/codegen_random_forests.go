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

package pai

import (
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func getTrainRandomForestsPAICmd(ir *ir.TrainStmt, session *pb.Session) (string, error) {
	// default use numTrees = 1
	treeNum := 1
	treeNumAttr, ok := ir.Attributes["tree_num"]
	if ok {
		treeNum = treeNumAttr.(int)
	}
	featureCols := []string{}
	for _, fclist := range ir.Features {
		for _, fc := range fclist {
			featureCols = append(featureCols, fc.GetFieldDesc()[0].Name)
		}
	}

	inputTables, e := formatODPSTables(ir.TmpTrainTable)
	if e != nil {
		return "", e
	}

	return fmt.Sprintf(`pai -name randomforests -DinputTableName="%s" -DmodelName="%s" -DlabelColName="%s" -DfeatureColNames="%s" -DtreeNum="%d"`,
		inputTables, ir.Into, ir.Label.GetFieldDesc()[0].Name, strings.Join(featureCols, ","), treeNum), nil
}

func getPredictRandomForestsPAICmd(ir *ir.PredictStmt, session *pb.Session) (string, error) {
	// NOTE(typhoonzero): for PAI random forests predicting, we can not load the TrainStmt
	// since the model saving is fully done by PAI. We directly use the columns in SELECT
	// statement for prediction, error will be reported by PAI job if the columns not match.
	db, err := database.OpenAndConnectDB(session.DbConnStr)
	if err != nil {
		return "", err
	}
	flds, _, err := getColumnTypes(ir.Select, db)
	if err != nil {
		return "", err
	}
	// drop result table if exists
	db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", ir.ResultTable))

	return fmt.Sprintf(`pai -name prediction -DmodelName="%s" -DinputTableName="%s" -DoutputTable="%s" -DfeatureColNames="%s"`,
		ir.Using, ir.TmpPredictTable, ir.ResultTable, strings.Join(flds, ",")), nil
}

func getExplainRandomForestsPAICmd(ir *ir.ExplainStmt, session *pb.Session) (string, error) {
	// NOTE(typhoonzero): for PAI random forests predicting, we can not load the TrainStmt
	// since the model saving is fully done by PAI. We directly use the columns in SELECT
	// statement for prediction, error will be reported by PAI job if the columns not match.
	db, err := database.OpenAndConnectDB(session.DbConnStr)
	if err != nil {
		return "", err
	}
	flds, _, err := getColumnTypes(ir.Select, db)
	if err != nil {
		return "", err
	}
	// drop result table if exists
	db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", ir.Into))
	labelCol, ok := ir.Attributes["label_column"]
	if !ok {
		return "", fmt.Errorf("must specify WITH label_column when using pai random forest to explain models")
	}
	featureFileds := []string{}
	for _, f := range flds {
		if f != labelCol {
			featureFileds = append(featureFileds, f)
		}
	}
	return fmt.Sprintf(`pai -name feature_importance -project algo_public -DmodelName="%s" -DinputTableName="%s"  -DoutputTableName="%s" -DlabelColName="%s" -DfeatureColNames="%s"`,
		ir.ModelName, ir.TmpExplainTable, ir.Into, labelCol.(string), strings.Join(featureFileds, ",")), nil
}
