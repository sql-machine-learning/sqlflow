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

func getTrainKMeansPAICmd(ir *ir.TrainStmt, session *pb.Session) (string, error) {
	centerCount := 3
	centerCountAttr, ok := ir.Attributes["center_count"]
	if ok {
		centerCount = centerCountAttr.(int)
	}
	featureCols := []string{}
	for _, fclist := range ir.Features {
		for _, fc := range fclist {
			featureCols = append(featureCols, fc.GetFieldDesc()[0].Name)
		}
	}
	idxTableName, ok := ir.Attributes["idx_table_name"]
	if !ok {
		return "", fmt.Errorf(`should set "idx_table_name" in WITH clause`)
	}
	db, err := database.OpenAndConnectDB(session.DbConnStr)
	if err != nil {
		return "", err
	}
	_, e := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", idxTableName))
	if e != nil {
		return "", e
	}

	return fmt.Sprintf(`pai -name kmeans -project algo_public -DinputTableName=%s -DcenterCount=%d -DmodelName %s -DidxTableName=%s -DselectedColNames=%s -DappendColNames="%s"`,
		ir.TmpTrainTable, centerCount, ir.Into, idxTableName, strings.Join(featureCols, ","), strings.Join(featureCols, ",")), nil
}
