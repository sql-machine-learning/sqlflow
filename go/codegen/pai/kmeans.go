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

	"sqlflow.org/sqlflow/go/attribute"
	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

var kmeansAttributes = attribute.Dictionary{}.
	Int("center_count", 3, `[default=3]
The cluster count. range: [1, Infinity]
`, attribute.IntLowerBoundChecker(1, true)).
	String("idx_table_name", "", `
The output table name which includes
cluster_index column indicates the cluster result,
distance column indicates the distance from the center and
all the columns of input table.`, nil).
	String("excluded_columns", "", `[default=""]
excluded the special feature columns from the SELECT statement.`, nil)

// InitializeKMeansAttributes initializes the attributes of KMeans and does type checking for them
func InitializeKMeansAttributes(trainStmt *ir.TrainStmt) error {
	kmeansAttributes.ExportDefaults(trainStmt.Attributes)
	return kmeansAttributes.Validate(trainStmt.Attributes)
}

func parseExcludedColsMap(attrs map[string]interface{}) map[string]int {
	excludedColsMap := make(map[string]int)
	excludedColsAttr := attrs["excluded_columns"].(string)
	if excludedColsAttr != "" {
		arr := strings.Split(excludedColsAttr, ",")
		for _, e := range arr {
			excludedColsMap[e] = 1
		}
	}
	return excludedColsMap
}

func getTrainKMeansPAICmd(ir *ir.TrainStmt, session *pb.Session) (string, error) {
	kmeansAttributes.ExportDefaults(ir.Attributes)
	if e := kmeansAttributes.Validate(ir.Attributes); e != nil {
		return "", e
	}
	centerCount := ir.Attributes["center_count"].(int)
	idxTableName := ir.Attributes["idx_table_name"].(string)
	if idxTableName == "" {
		return "", fmt.Errorf(`should set "idx_table_name" in WITH clause`)
	}

	excludedColsMap := parseExcludedColsMap(ir.Attributes)

	// featureCols indicates feature columns used to append to the output table
	featureCols := []string{}
	// selectedCols indicates feature columns used to clustering
	selectedCols := []string{}
	for _, fclist := range ir.Features {
		for _, fc := range fclist {
			fcName := fc.GetFieldDesc()[0].Name
			featureCols = append(featureCols, fcName)
			if _, ok := excludedColsMap[fcName]; !ok {
				selectedCols = append(selectedCols, fcName)
			}
		}
	}

	db, e := database.OpenAndConnectDB(session.DbConnStr)
	if e != nil {
		return "", e
	}
	defer db.Close()
	_, e = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", idxTableName))
	if e != nil {
		return "", e
	}

	return fmt.Sprintf(`pai -name kmeans -project algo_public -DinputTableName=%s -DcenterCount=%d -DmodelName %s -DidxTableName=%s -DselectedColNames=%s -DappendColNames="%s"`,
		ir.TmpTrainTable, centerCount, ir.Into, idxTableName, strings.Join(selectedCols, ","), strings.Join(featureCols, ",")), nil
}
