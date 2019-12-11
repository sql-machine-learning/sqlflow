// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

package ir

import (
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func mockSession() *pb.Session {
	return &pb.Session{
		Token:            "",
		DbConnStr:        "",
		ExitOnSubmit:     false,
		UserId:           "",
		HiveLocation:     "/sqlflowtmp",
		HdfsNamenodeAddr: "192.168.1.1:8020",
		HdfsUser:         "sqlflow_admin",
		HdfsPass:         "sqlflow_pass",
	}
}

const testDataSource = "mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"

func TestTrainProto(t *testing.T) {
	a := assert.New(t)
	sampleTrainStmt := MockTrainStmt(testDataSource, false)
	pbIR, err := TrainStmtToProto(sampleTrainStmt, mockSession())
	a.NoError(err)
	pbtxt := proto.MarshalTextString(pbIR)
	pbIRToTest := &pb.TrainStmt{}
	err = proto.UnmarshalText(pbtxt, pbIRToTest)
	a.NoError(err)
	a.Equal(
		sampleTrainStmt.Features["feature_columns"][2].GetFieldMeta()[0].Name,
		pbIRToTest.GetFeatures()["feature_columns"].GetFeatureColumns()[2].GetNc().GetFieldMeta().GetName(),
	)
	a.Equal(
		int32(sampleTrainStmt.Attributes["train.batch_size"].(int)),
		pbIRToTest.GetAttributes()["train.batch_size"].GetI(),
	)
}

func TestPredictProto(t *testing.T) {
	a := assert.New(t)
	samplePredStmt := MockPredStmt(MockTrainStmt(testDataSource, false))
	pbIR, err := PredictStmtToProto(samplePredStmt, mockSession())
	a.NoError(err)
	pbtxt := proto.MarshalTextString(pbIR)
	pbIRToTest := &pb.PredictStmt{}
	err = proto.UnmarshalText(pbtxt, pbIRToTest)
	a.NoError(err)
	a.Equal(
		samplePredStmt.ResultTable,
		pbIRToTest.GetResultTable(),
	)
}

func TestAnalyzeProto(t *testing.T) {
	a := assert.New(t)
	sampleAnalyzeStmt := &AnalyzeStmt{
		ExtendedSQL: ExtendedSQL{"select * from iris.train;", testDataSource, map[string]interface{}{}, ""},
		Explainer:   "TreeExplainer",
		TrainStmt:   MockTrainStmt(testDataSource, true),
	}
	pbIR, err := AnalyzeStmtToProto(sampleAnalyzeStmt, mockSession())
	a.NoError(err)
	pbtxt := proto.MarshalTextString(pbIR)
	pbIRToTest := &pb.AnalyzeStmt{}
	err = proto.UnmarshalText(pbtxt, pbIRToTest)
	a.NoError(err)
	a.Equal(
		sampleAnalyzeStmt.Explainer,
		pbIRToTest.GetExplainer(),
	)
}
