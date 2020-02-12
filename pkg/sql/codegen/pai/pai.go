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

func getPAIPredictCmd(ir *ir.PredictStmt, session *pb.Session) (string, error) {
	// NOTE(typhoonzero): for PAI machine learning tookit predicting, we can not load the TrainStmt
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

	return fmt.Sprintf(`pai -name prediction -DmodelName="%s" -DinputTableName="%s" -DoutputTableName="%s" -DfeatureColNames="%s" -DappendColNames="%s"`,
		ir.Using, ir.TmpPredictTable, ir.ResultTable, strings.Join(flds, ","), strings.Join(flds, ",")), nil
}
