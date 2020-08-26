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
	"encoding/binary"
	"fmt"
	"github.com/bitly/go-simplejson"
	"net/url"
	"sqlflow.org/sqlflow/go/model"
	"sqlflow.org/sqlflow/go/sqlfs"
	"strings"

	"sqlflow.org/sqlflow/go/database"

	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

func generateStepCodeAndImage(sqlStmt ir.SQLFlowStmt, stepIndex int, session *pb.Session, sqlStmts []ir.SQLFlowStmt) (string, string, error) {
	switch stmt := sqlStmt.(type) {
	case *ir.TrainStmt:
		return generateTrainCodeAndImage(stmt, stepIndex, session)
	case *ir.PredictStmt:
		return generatePredictCodeAndImage(stmt, stepIndex, session, sqlStmts)
	case *ir.EvaluateStmt:
		return generateEvaluationCodeAndImage(stmt, stepIndex, session, sqlStmts)
	case *ir.NormalStmt:
		code, err := generateNormalStmtStep(string(*stmt), stepIndex, session)
		return code, "", err
	default:
		return "", "", fmt.Errorf("not implemented stmt execution type %v", stmt)
	}
}

func generateTrainCodeAndImage(trainStmt *ir.TrainStmt, stepIndex int, session *pb.Session) (string, string, error) {
	isXGBoost := isXGBoostEstimator(trainStmt.Estimator)
	if isXGBoost {
		code, err := XGBoostGenerateTrain(trainStmt, stepIndex, session)
		if err != nil {
			return "", "", err
		}
		return code, trainStmt.ModelImage, nil
	}
	return "", "", fmt.Errorf("not implemented estimator type %s", trainStmt.Estimator)
}

func generatePredictCodeAndImage(predStmt *ir.PredictStmt, stepIndex int, session *pb.Session, sqlStmts []ir.SQLFlowStmt) (string, string, error) {
	image := ""
	isXGBoost := false
	trainStmt := findModelGenerationTrainStmt(predStmt.Using, stepIndex, sqlStmts)
	if trainStmt != nil {
		image = trainStmt.ModelImage
		isXGBoost = isXGBoostEstimator(trainStmt.Estimator)
	} else {
		meta, err := getModelMetadata(session, predStmt.Using)
		if err != nil {
			return "", "", err
		}
		image = meta.imageName()
		isXGBoost = meta.isXGBoostModel()
	}

	if isXGBoost {
		code, err := XGBoostGeneratePredict(predStmt, stepIndex, session)
		if err != nil {
			return "", "", err
		}
		return code, image, nil
	}
	return "", "", fmt.Errorf("not implemented model type")
}

func generateEvaluationCodeAndImage(evalStmt *ir.EvaluateStmt, stepIndex int, session *pb.Session, sqlStmts []ir.SQLFlowStmt) (string, string, error) {
	image := ""
	isXGBoost := false
	trainStmt := findModelGenerationTrainStmt(evalStmt.ModelName, stepIndex, sqlStmts)
	if trainStmt != nil {
		image = trainStmt.ModelImage
		isXGBoost = isXGBoostEstimator(trainStmt.Estimator)
	} else {
		meta, err := getModelMetadata(session, evalStmt.ModelName)
		if err != nil {
			return "", "", err
		}
		image = meta.imageName()
		isXGBoost = meta.isXGBoostModel()
	}

	if isXGBoost {
		code, err := XGBoostGenerateEvaluation(evalStmt, stepIndex, session)
		if err != nil {
			return "", "", err
		}
		return code, image, nil
	}
	return "", "", fmt.Errorf("not implemented model type")
}

// findModelGenerationTrainStmt finds the *ir.TrainStmt that generates the model named `modelName`.
// TODO(sneaxiy): find a better way to do this when we have a well designed dependency analysis.
func findModelGenerationTrainStmt(modelName string, idx int, sqlStmts []ir.SQLFlowStmt) *ir.TrainStmt {
	idx--
	for idx >= 0 {
		trainStmt, ok := sqlStmts[idx].(*ir.TrainStmt)
		if ok && trainStmt.Into == modelName {
			return trainStmt
		}
		idx--
	}
	return nil
}

func isXGBoostEstimator(estimator string) bool {
	return strings.HasPrefix(strings.ToUpper(estimator), "XGBOOST.")
}

type metadata simplejson.Json

func (m *metadata) imageName() string {
	return (*simplejson.Json)(m).Get("model_repo_image").MustString()
}

func (m *metadata) isXGBoostModel() bool {
	return (*simplejson.Json)(m).Get("model_type").MustInt() == model.XGBOOST
}

func getModelMetadata(session *pb.Session, table string) (*metadata, error) {
	submitter := getSubmitter(session)
	if submitter == "local" {
		return getModelMetadataFromDB(session.DbConnStr, table)
	}
	return nil, fmt.Errorf("not supported submitter %s", submitter)
}

func getModelMetadataFromDB(dbConnStr, table string) (*metadata, error) {
	db, err := database.OpenAndConnectDB(dbConnStr)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	fs, err := sqlfs.Open(db.DB, table)
	if err != nil {
		return nil, err
	}
	defer fs.Close()

	lengthBytes := make([]byte, 8)
	readCnt, err := fs.Read(lengthBytes)
	if err != nil {
		return nil, err
	}
	if readCnt != 8 {
		return nil, fmt.Errorf("invalid model table")
	}

	length := binary.LittleEndian.Uint64(lengthBytes)
	jsonBytes := make([]byte, length)
	readCnt, err = fs.Read(jsonBytes)
	if err != nil {
		return nil, err
	}
	if uint64(readCnt) != length {
		return nil, fmt.Errorf("invalid model metadata")
	}
	json, err := simplejson.NewJson(jsonBytes)
	if err != nil {
		return nil, err
	}
	return (*metadata)(json), nil
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
