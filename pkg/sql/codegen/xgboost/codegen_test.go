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

package xgboost

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/ir"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func TestAttributes(t *testing.T) {
	a := assert.New(t)
	a.Equal(6, len(attributeDictionary))
	a.Equal(29, len(fullAttrValidator))

	a.Error(objectiveChecker("binaray:logistic"))
	a.NoError(objectiveChecker("binary:logistic"))
}

func mockSession() *pb.Session {
	return &pb.Session{DbConnStr: "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"}
}

func TestTrainAndPredict(t *testing.T) {
	a := assert.New(t)
	tir := ir.MockTrainStmt(true)
	_, err := Train(tir, mockSession())
	a.NoError(err)

	pir := ir.MockPredStmt(tir)
	sess := &pb.Session{
		Token:            "",
		DbConnStr:        "",
		ExitOnSubmit:     false,
		UserId:           "",
		HiveLocation:     "/sqlflowtmp",
		HdfsNamenodeAddr: "192.168.1.1:8020",
		HdfsUser:         "sqlflow_admin",
		HdfsPass:         "sqlflow_pass",
	}
	code, err := Pred(pir, sess)

	r, _ := regexp.Compile(`hdfs_user='''(.*)'''`)
	a.Equal(r.FindStringSubmatch(code)[1], "sqlflow_admin")
	r, _ = regexp.Compile(`hdfs_pass='''(.*)'''`)
	a.Equal(r.FindStringSubmatch(code)[1], "sqlflow_pass")

	a.NoError(err)
}
