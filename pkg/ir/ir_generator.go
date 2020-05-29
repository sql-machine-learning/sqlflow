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

package ir

import (
	"sqlflow.org/sqlflow/pkg/parser"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"strings"
)

// GenerateStatement generates a `pb.Statement` from `parser.SQLFlowSelectStmt`
func GenerateStatement(sql *parser.SQLFlowStmt) (*pb.Statement, error) {
	stmt := &pb.Statement{}
	stmt.OriginalSql = sql.Original
	slct := sql.SQLFlowSelectStmt
	if slct == nil || !slct.Extended {
		stmt.Type = pb.Statement_QUERY
		stmt.Select = sql.Original
		return stmt, nil
	}
	stmt.Attributes = map[string]string{}
	stmt.Columns = map[string]*pb.Statement_Columns{}
	stmt.Select = slct.StandardSelect.String()
	if slct.Train {
		tc := slct.TrainClause
		stmt.Type = pb.Statement_TRAIN
		modelURI := tc.Estimator
		// get model Docker image name
		modelParts := strings.Split(modelURI, "/")
		modelImageName := strings.Join(modelParts[0:len(modelParts)-1], "/")
		modelName := modelParts[len(modelParts)-1]
		stmt.Attributes = map[string]string{}
		for k, v := range tc.TrainAttrs {
			stmt.Attributes[k] = v.String()
		}
		for k, v := range tc.Columns {
			stmt.Columns[k] = &pb.Statement_Columns{}
			for _, expr := range v {
				stmt.Columns[k].Columns = append(stmt.Columns[k].Columns, expr.String())
			}
		}
		stmt.Label = tc.Label
		stmt.Estimator = modelName
		stmt.ModelImage = modelImageName
		stmt.ModelSave = slct.Save
	} else if slct.Predict {
		c := slct.PredictClause
		stmt.Type = pb.Statement_PREDICT
		stmt.Attributes = map[string]string{}
		for k, v := range c.PredAttrs {
			stmt.Attributes[k] = v.String()
		}
		stmt.Target = c.Into
		stmt.ModelSave = c.Model
	} else if slct.Explain {
		c := slct.ExplainClause
		stmt.Type = pb.Statement_EXPLAIN
		stmt.Attributes = map[string]string{}
		for k, v := range c.ExplainAttrs {
			stmt.Attributes[k] = v.String()
		}
		stmt.Target = c.ExplainInto
		stmt.ModelSave = c.TrainedModel
	} else if slct.Evaluate {
		c := slct.EvaluateClause
		stmt.Type = pb.Statement_EVALUATE
		stmt.Attributes = map[string]string{}
		for k, v := range c.EvaluateAttrs {
			stmt.Attributes[k] = v.String()
		}
		stmt.Target = c.EvaluateInto
		stmt.ModelSave = c.ModelToEvaluate
	} else if slct.ShowTrain {
		c := slct.ShowTrainClause
		stmt.Type = pb.Statement_SHOW
		stmt.ModelSave = c.ModelName
	}
	return stmt, nil
}
