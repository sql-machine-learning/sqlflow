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

package experimental

import (
	"fmt"
	"net/url"
	"sqlflow.org/sqlflow/go/database"
	"strings"

	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

// TODO(sneaxiy): implement this method to distinguish whether
// a model is a XGBoost model.
func isTrainedXBoostModel(modelName string) bool {
	return true
}

func generateStepCode(sqlStmt ir.SQLFlowStmt, stepIndex int, session *pb.Session) (string, error) {
	switch stmt := sqlStmt.(type) {
	case *ir.TrainStmt:
		if strings.HasPrefix(strings.ToUpper(stmt.Estimator), "XGBOOST.") {
			return XGBoostGenerateTrain(stmt, stepIndex, session)
		}
		return "", fmt.Errorf("not implemented estimator type %s", stmt.Estimator)
	case *ir.PredictStmt:
		if isTrainedXBoostModel(stmt.Using) {
			return XGBoostGeneratePredict(stmt, stepIndex, session)
		}
		return "", fmt.Errorf("not implemented model type")
	case *ir.NormalStmt:
		return GenerateNormalStmtStep(string(*stmt), session, stepIndex)
	default:
		return "", fmt.Errorf("not implemented stmt execution type %v", stmt)
	}
}

func initializeAndCheckAttributes(stmt ir.SQLFlowStmt) error {
	switch s := stmt.(type) {
	case *ir.TrainStmt:
		if s.GetModelKind() == ir.XGBoost {
			return InitializeAttributes(s)
		}
		// TODO(typhoonzero): add below lines
		// 	else if s.GetModelKind() == ir.KMeans {
		// 		return pai.InitializeKMeansAttributes(s)
		// 	}
		// 	return tensorflow.InitializeAttributes(s)
		// case *ir.OptimizeStmt:
		// 	return optimize.InitializeAttributes(s)
	}
	return nil
}

// InitializeAttributes initializes the attributes of XGBoost and does type checking for them
func InitializeAttributes(trainStmt *ir.TrainStmt) error {
	attributeDictionary.ExportDefaults(trainStmt.Attributes)
	return fullAttrValidator.Validate(trainStmt.Attributes)
}

// GeneratePyDbConnStr generates the db connection string for the Python dbapi.
func GeneratePyDbConnStr(session *pb.Session) (string, error) {
	dialect, _, err := database.ParseURL(session.DbConnStr)
	if err != nil {
		return "", err
	}

	if dialect != "hive" {
		return session.DbConnStr, nil
	}

	u, err := url.Parse(session.DbConnStr)
	if err != nil {
		return "", err
	}

	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", err
	}

	query.Set("hdfs_namenode_addr", session.HdfsNamenodeAddr)
	query.Set("hive_location", session.HiveLocation)
	query.Set("hdfs_user", session.HdfsUser)
	query.Set("hdfs_pass", session.HdfsPass)

	u.RawQuery = query.Encode()
	return u.String(), nil
}
