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
	"os"
	"regexp"
	"sqlflow.org/sqlflow/go/model"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"sqlflow.org/sqlflow/go/codegen/tensorflow"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

var dataSource = "maxcompute://test:test@service-maxcompute.com/api?curr_project=test&scheme=http"

var exportedLocal = []string{
	"feature_columns",
	"feature_column_names",
	"feature_metas",
}

var knownTrainParams = append(
	[]string{
		"is_keras_model",
		"datasource",
		"estimator",
		"select",
		"validate_select",
		"save",
		"batch_size",
		"epochs",
		"verbose",
	},
	exportedLocal...)

var knownPredictParams = append(
	[]string{
		"result_table",
		"hdfs_namenode_addr",
		"hive_location",
		"hdfs_user",
		"hdfs_pass",
	},
	knownTrainParams...,
)

func contains(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

func hasExportedLocal(code string) bool {
	for _, v := range exportedLocal {
		r := regexp.MustCompile(fmt.Sprintf(`\b%[1]s=%[1]s,`, v))
		if len(r.FindStringIndex(code)) <= 0 {
			return false
		}
	}
	return true
}

func hasUnknownParameters(code string, list []string) bool {
	r := regexp.MustCompile(`(?s)((?:\b_predict\(|\bpred\(|\btrain\().*)`)
	c := r.FindStringSubmatch(code)[1]
	r = regexp.MustCompile(`[(\s,](\w+)=.*,`)
	for _, v := range r.FindStringSubmatch(c)[1:] {
		if !contains(list, v) {
			return true
		}
	}
	return false
}

func mockClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		PS: PSConfig{
			Count: 0,
			CPU:   200,
			GPU:   0,
		},
		Worker: WorkerConfig{
			Count: 0,
			CPU:   200,
			GPU:   0,
		},
	}
}

func mockSession() *pb.Session {
	return &pb.Session{
		DbConnStr: "maxcompute://root:root@maxcompute.xxx.com?curr_project=project&scheme=http",
		UserId:    "sqlflow",
	}
}

func TestTrainCodegen(t *testing.T) {
	a := assert.New(t)
	trainStmt := ir.MockTrainStmt(false)

	os.Setenv("SQLFLOW_OSS_CHECKPOINT_CONFIG", "{\"host\": \"h.com\", \"arn\": \"acs:ram::9527:role\"}")
	defer os.Unsetenv("SQLFLOW_OSS_CHECKPOINT_CONFIG")

	sess := mockSession()
	ossModelPath := "iris/sqlflow/my_dnn_model"
	scriptPath := "file:///tmp/task.tar.gz"
	paramsPath := "file:///tmp/params.txt"
	paiTFCode, paiCmd, _, e := Train(trainStmt, sess, scriptPath, paramsPath, "my_dnn_model", ossModelPath, "", "")
	a.NoError(e)

	tfCode, err := tensorflow.Train(trainStmt, sess)
	a.NoError(err)

	a.True(strings.Contains(paiTFCode, tfCode))
	a.True(hasExportedLocal(tfCode))
	a.False(hasUnknownParameters(paiTFCode, knownTrainParams))

	expectedPAICmd := fmt.Sprintf("pai -name tensorflow1150 -project algo_public_dev -DmaxHungTimeBeforeGCInSeconds=0 -DjobName=sqlflow_my_dnn_model -Dtags=dnn -Dscript=%s -DentryFile=entry.py -Dtables=odps://iris/tables/train,odps://iris/tables/test  -DhyperParameters=\"%s\" -DcheckpointDir='oss://sqlflow-models/iris/sqlflow/my_dnn_model/?role_arn=acs:ram::9527:role/pai2ossproject&host=h.com' -DgpuRequired='0'", scriptPath, paramsPath)
	a.Equal(expectedPAICmd, paiCmd)
}

func TestPredictCodegen(t *testing.T) {
	a := assert.New(t)
	ir := ir.MockPredStmt(ir.MockTrainStmt(false))

	os.Setenv("SQLFLOW_OSS_CHECKPOINT_CONFIG", "{\"host\": \"h.com\", \"arn\": \"acs:ram::9527:role\"}")
	defer os.Unsetenv("SQLFLOW_OSS_CHECKPOINT_CONFIG")
	sess := mockSession()
	ossModelPath := "iris/sqlflow/my_dnn_model"
	scriptPath := "file:///tmp/task.tar.gz"
	paramsPath := "file:///tmp/params.txt"
	paiTFCode, paiCmd, _, e := Predict(ir, sess, scriptPath, paramsPath, "my_dnn_model", ossModelPath, "", model.TENSORFLOW)
	a.NoError(e)
	a.False(hasUnknownParameters(paiTFCode, knownPredictParams))
	tfCode, err := tensorflow.Pred(ir, sess)
	a.NoError(err)

	a.True(hasExportedLocal(tfCode))
	a.False(hasUnknownParameters(tfCode, knownPredictParams))
	expectedPAICmd := fmt.Sprintf("pai -name tensorflow1150 -project algo_public_dev -DmaxHungTimeBeforeGCInSeconds=0 -DjobName=sqlflow_my_dnn_model -Dtags=dnn -Dscript=%s -DentryFile=entry.py -Dtables=odps://iris/tables/predict -Doutputs=odps://iris/tables/predict -DhyperParameters=\"%s\" -DcheckpointDir='oss://sqlflow-models/iris/sqlflow/my_dnn_model/?role_arn=acs:ram::9527:role/pai2ossproject&host=h.com' -DgpuRequired='0'", scriptPath, paramsPath)
	a.Equal(expectedPAICmd, paiCmd)
}
