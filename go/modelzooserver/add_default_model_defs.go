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

package modelzooserver

import (
	"fmt"

	"sqlflow.org/sqlflow/go/database"
)

func addDefaultModelDefs(mysqlConn *database.DB) error {
	findModelRepoStmt := fmt.Sprintf("SELECT * FROM %s WHERE name='sqlflow/sqlflow' AND version='latest';", modelCollTable)
	rows, err := mysqlConn.DB.Query(findModelRepoStmt)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasNext := rows.Next()
	if hasNext {
		// default model defs already added, do not add again.
		return nil
	}
	addModelRepoStmt := fmt.Sprintf("INSERT INTO %s (name, version) VALUES ('sqlflow/sqlflow', 'latest');", modelCollTable)
	addModelRepoRes, err := mysqlConn.DB.Exec(addModelRepoStmt)
	if err != nil {
		return err
	}
	modelRepoID, err := addModelRepoRes.LastInsertId()
	if err != nil {
		return err
	}
	defaultModelNames := []string{
		"DNNClassifier", "DNNRegressor", "LinearClassifier", "LinearRegressor",
		"BoostedTreesClassifier", "BoostedTreesRegressor", "DNNLinearCombinedClassifier",
		"DNNLinearCombinedRegressor", "sqlflow_models.DNNClassifier", "sqlflow_models.DNNRegressor",
		"sqlflow_models.StackedRNNClassifier", "sqlflow_models.DeepEmbeddingClusterModel",
		"sqlflow_models.RNNBasedTimeSeriesModel", "sqlflow_models.AutoClassifier",
		"sqlflow_models.AutoRegressor", "sqlflow_models.RawDNNClassifier",
		"sqlflow_models.ARIMAWithSTLDecomposition",
	}
	for _, modelName := range defaultModelNames {
		addModelDefStmt := fmt.Sprintf(
			"INSERT INTO %s (model_coll_id, class_name, args_desc) VALUES (%d, '%s', '%s')",
			modelDefTable, modelRepoID, modelName, "SQLFlow predefined model, please refer to the documentation for details: https://github.com/sql-machine-learning/sqlflow/blob/v0.3.0-rc.1/doc/sqlflow.org_cn/model_list.md")
		if _, err := mysqlConn.DB.Exec(addModelDefStmt); err != nil {
			return err
		}
	}
	return nil
}
