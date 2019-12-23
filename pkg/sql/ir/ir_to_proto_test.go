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
	"sqlflow.org/sqlflow/pkg/database"
	pb "sqlflow.org/sqlflow/pkg/proto"
)

func TestTrainProto(t *testing.T) {
	a := assert.New(t)
	sampleTrainStmt := MockTrainStmt(database.MockURL(), false)
	pbIR, err := TrainStmtToProto(sampleTrainStmt, database.MockSession())
	a.NoError(err)
	pbtxt := proto.MarshalTextString(pbIR)
	pbIRToTest := &pb.TrainStmt{}
	err = proto.UnmarshalText(pbtxt, pbIRToTest)
	a.NoError(err)
	a.Equal(
		sampleTrainStmt.Features["feature_columns"][2].GetFieldDesc()[0].Name,
		pbIRToTest.GetFeatures()["feature_columns"].GetFeatureColumns()[2].GetNc().GetFieldDesc().GetName(),
	)
	a.Equal(
		int32(sampleTrainStmt.Attributes["train.batch_size"].(int)),
		pbIRToTest.GetAttributes()["train.batch_size"].GetI(),
	)
}

func TestPredictProto(t *testing.T) {
	a := assert.New(t)
	samplePredStmt := MockPredStmt(MockTrainStmt(database.MockURL(), false))
	pbIR, err := PredictStmtToProto(samplePredStmt, database.MockSession())
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
		DataSource: database.MockURL(),
		Select:     "select * from iris.train;",
		Attributes: make(map[string]interface{}), // empty attribute
		Explainer:  "TreeExplainer",
		TrainStmt:  MockTrainStmt(database.MockURL(), true),
	}
	pbIR, err := AnalyzeStmtToProto(sampleAnalyzeStmt, database.MockSession())
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
