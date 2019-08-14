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

package sql

import (
	"testing"

	pb "github.com/sql-machine-learning/sqlflow/server/proto"
	"github.com/stretchr/testify/assert"
)

func TestTrainElasticDLFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	// TODO: Currently we use ALPS' parameter imm WITH block temporarily
	// Need to register for ElasticDL parameters
	wndStatement := `select c1, c2, c3, c4, c5 from training_data 
		TRAIN ElasticDLKerasClassifier 
		WITH
			engine.type = "k8s"
		COLUMN
			c1,
			NUMERIC(c2, 10),
			c3,
			c4
		LABEL c5
		INTO trained_elasticdl_keras_classifier;`

	r, e := parser.Parse(wndStatement)
	a.NoError(e)
	session := &pb.Session{UserId: "sqlflow_user"}
	filler, e := newElasticDLTrainFiller(r, nil, session, nil)
	a.NoError(e)

	a.True(filler.IsTraining)
	a.Equal("training_data", filler.TrainInputTable)
}
