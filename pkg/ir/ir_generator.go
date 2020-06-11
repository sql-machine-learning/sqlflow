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
	stmt := &pb.Statement{OriginalSql: sql.Original}
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
		stmt.Type = pb.Statement_TRAIN
		modelURI := slct.Estimator
		// get model Docker image name
		modelParts := strings.Split(modelURI, "/")
		modelImageName := strings.Join(modelParts[0:len(modelParts)-1], "/")
		modelName := modelParts[len(modelParts)-1]
		for k, v := range slct.TrainAttrs {
			stmt.Attributes[k] = v.String()
		}
		for k, v := range slct.Columns {
			stmt.Columns[k] = &pb.Statement_Columns{}
			for _, expr := range v {
				stmt.Columns[k].Columns = append(stmt.Columns[k].Columns, expr.String())
			}
		}
		stmt.Label = slct.Label
		stmt.Estimator = modelName
		stmt.ModelImage = modelImageName
		stmt.ModelSave = slct.Save
		stmt.TrainedModel = slct.TrainUsing
	} else if slct.Predict {
		stmt.Type = pb.Statement_PREDICT
		for k, v := range slct.PredAttrs {
			stmt.Attributes[k] = v.String()
		}
		stmt.Target = slct.Into
		stmt.ModelSave = slct.Model
	} else if slct.Explain {
		stmt.Type = pb.Statement_EXPLAIN
		for k, v := range slct.ExplainAttrs {
			stmt.Attributes[k] = v.String()
		}
		stmt.Target = slct.ExplainInto
		stmt.ModelSave = slct.TrainedModel
	} else if slct.Evaluate {
		stmt.Type = pb.Statement_EVALUATE
		for k, v := range slct.EvaluateAttrs {
			stmt.Attributes[k] = v.String()
		}
		stmt.Target = slct.EvaluateInto
		stmt.ModelSave = slct.ModelToEvaluate
	} else if slct.Optimize {
		stmt.Type = pb.Statement_OPTIMIZE
	} else if slct.ShowTrain {
		stmt.Type = pb.Statement_SHOW
		stmt.ModelSave = slct.ModelName
	}
	return stmt, nil
}
