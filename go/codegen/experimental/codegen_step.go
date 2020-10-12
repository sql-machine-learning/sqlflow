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
	"context"
	"fmt"
	"google.golang.org/grpc"
	"net/url"
	"os"
	"strconv"
	"strings"

	"sqlflow.org/sqlflow/go/codegen/tensorflow"

	"github.com/bitly/go-simplejson"
	"sqlflow.org/sqlflow/go/sqlfs"

	"sqlflow.org/sqlflow/go/database"

	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

// GenerateStepCodeAndImage generates step code and image
func GenerateStepCodeAndImage(sqlStmt ir.SQLFlowStmt, stepIndex int, session *pb.Session, sqlStmts []ir.SQLFlowStmt) (string, string, error) {
	switch stmt := sqlStmt.(type) {
	case *ir.TrainStmt:
		return generateTrainCodeAndImage(stmt, stepIndex, session)
	case *ir.PredictStmt:
		return generatePredictCodeAndImage(stmt, stepIndex, session, sqlStmts)
	case *ir.EvaluateStmt:
		return generateEvaluationCodeAndImage(stmt, stepIndex, session, sqlStmts)
	case *ir.ExplainStmt:
		return generateExplainCodeAndImage(stmt, stepIndex, session, sqlStmts)
	case *ir.ShowTrainStmt:
		code, err := generateShowTrainCode(stmt, stepIndex, session)
		return code, "", err
	case *ir.OptimizeStmt:
		code, err := generateOptimizeCode(stmt, stepIndex, session)
		return code, "", err
	case *ir.NormalStmt:
		code, err := generateNormalStmtStep(string(*stmt), stepIndex, session)
		return code, "", err
	default:
		return "", "", fmt.Errorf("not implemented stmt execution type %v", stmt)
	}
}

func generateTrainCodeAndImage(trainStmt *ir.TrainStmt, stepIndex int, session *pb.Session) (string, string, error) {
	code, err := GenerateTrain(trainStmt, stepIndex, session)
	if err != nil {
		return "", "", err
	}
	return code, trainStmt.ModelImage, nil
}

func generatePredictCodeAndImage(predStmt *ir.PredictStmt, stepIndex int, session *pb.Session, sqlStmts []ir.SQLFlowStmt) (string, string, error) {
	image := ""
	trainStmt := findModelGenerationTrainStmt(predStmt.Using, stepIndex, sqlStmts)
	if trainStmt != nil {
		image = trainStmt.ModelImage
	} else {
		meta, err := getModelMetadata(session, predStmt.Using)
		if err != nil {
			return "", "", err
		}
		image = meta.imageName()
	}

	code, err := GeneratePredict(predStmt, stepIndex, session)
	if err != nil {
		return "", "", err
	}
	return code, image, nil
}

func generateEvaluationCodeAndImage(evalStmt *ir.EvaluateStmt, stepIndex int, session *pb.Session, sqlStmts []ir.SQLFlowStmt) (string, string, error) {
	image := ""
	trainStmt := findModelGenerationTrainStmt(evalStmt.ModelName, stepIndex, sqlStmts)
	if trainStmt != nil {
		image = trainStmt.ModelImage
	} else {
		meta, err := getModelMetadata(session, evalStmt.ModelName)
		if err != nil {
			return "", "", err
		}
		image = meta.imageName()
	}

	code, err := GenerateEvaluation(evalStmt, stepIndex, session)
	if err != nil {
		return "", "", err
	}
	return code, image, nil
}

func generateExplainCodeAndImage(explainStmt *ir.ExplainStmt, stepIndex int, session *pb.Session, sqlStmts []ir.SQLFlowStmt) (string, string, error) {
	image := ""
	trainStmt := findModelGenerationTrainStmt(explainStmt.ModelName, stepIndex, sqlStmts)
	if trainStmt != nil {
		image = trainStmt.ModelImage
	} else {
		meta, err := getModelMetadata(session, explainStmt.ModelName)
		if err != nil {
			return "", "", err
		}
		image = meta.imageName()
	}

	code, err := GenerateExplain(explainStmt, stepIndex, session)
	if err != nil {
		return "", "", err
	}
	return code, image, nil
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

// Metadata represents the metadata of a trained model
type Metadata simplejson.Json

func (m *Metadata) imageName() string {
	return (*simplejson.Json)(m).Get("model_repo_image").MustString()
}

func getModelMetadata(session *pb.Session, table string) (*Metadata, error) {
	submitter := getSubmitter(session)
	if submitter == "local" {
		modelZooAddr, table, tag := decomposeModelName(table)
		if modelZooAddr != "" {
			return getModelMetadataFromModelZoo(modelZooAddr, table, tag)
		}
		return GetModelMetadataFromDB(session.DbConnStr, table)
	}
	return nil, fmt.Errorf("not supported submitter %s", submitter)
}

func decomposeModelName(modelName string) (string, string, string) {
	idx := strings.LastIndex(modelName, "/")
	if idx < 0 {
		return "", modelName, ""
	}

	address := modelName[0:idx]
	modelName = modelName[idx+1:]
	idx = strings.LastIndex(modelName, ":")
	tag := ""
	if idx >= 0 {
		tag = modelName[idx+1:]
		modelName = modelName[0:idx]
	}
	return address, modelName, tag
}

func getModelMetadataFromModelZoo(addr, table, tag string) (*Metadata, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewModelZooServerClient(conn)
	req := &pb.ReleaseModelRequest{
		Name: table,
		Tag:  tag,
	}
	resp, err := client.GetModelMeta(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("error is from: %v %s", err, req.Name)
	}
	json, err := simplejson.NewJson([]byte(resp.Meta))
	if err != nil {
		return nil, err
	}
	return (*Metadata)(json), nil
}

// GetModelMetadataFromDB gets model Metadata from DBMS
func GetModelMetadataFromDB(dbConnStr, table string) (*Metadata, error) {
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

	lengthHexStr := make([]byte, 10)
	n, err := fs.Read(lengthHexStr)
	if err != nil || n != 10 {
		return nil, fmt.Errorf("read meta length from db error: %v", err)
	}
	metaLength, err := strconv.ParseInt(string(lengthHexStr), 0, 64)
	if err != nil {
		return nil, fmt.Errorf("convert length head error: %v", err)
	}
	jsonBytes := make([]byte, metaLength)
	l, err := fs.Read(jsonBytes)
	if err != nil {
		return nil, fmt.Errorf("read meta json from db error: %v", err)
	}
	if int64(l) != metaLength {
		return nil, fmt.Errorf("read meta json from db error: invalid meta length read %d", l)
	}

	json, err := simplejson.NewJson(jsonBytes)
	if err != nil {
		return nil, err
	}
	return (*Metadata)(json), nil
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
		return tensorflow.InitializeAttributes(s)
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

func getSubmitter(session *pb.Session) string {
	if session.Submitter != "" {
		return session.Submitter
	}

	submitter := os.Getenv("SQLFLOW_submitter")
	if submitter != "" {
		return submitter
	}
	return "local"
}

func generateFeatureColumnCode(fcMap map[string][]ir.FeatureColumn) string {
	allFCCodes := make([]string, 0)
	for target, fcList := range fcMap {
		if len(fcList) == 0 {
			continue
		}
		codeList := make([]string, 0)
		for _, fc := range fcList {
			codeList = append(codeList, fc.GenPythonCode())
		}
		code := fmt.Sprintf(`"%s":[%s]`, target, strings.Join(codeList, ","))
		allFCCodes = append(allFCCodes, code)
	}
	return fmt.Sprintf("{%s}", strings.Join(allFCCodes, ","))
}

func categorizeAttributes(attrs map[string]interface{}) map[string]map[string]interface{} {
	params := make(map[string]map[string]interface{})
	prefixList := []string{"train.", "model.", "validation."}
	for _, prefix := range prefixList {
		params[prefix] = make(map[string]interface{})
	}

	for k, v := range attrs {
		foundPrefix := false
		for _, prefix := range prefixList {
			if strings.HasPrefix(k, prefix) {
				params[prefix][k[len(prefix):]] = v
				foundPrefix = true
				break
			}
		}

		// all parameters without prefix are considered as
		// model.xxx
		if !foundPrefix {
			params["model."][k] = v
		}
	}
	return params
}
