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

package alps

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"sqlflow.org/sqlflow/go/codegen/tensorflow"
	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

// Train generates code to train a model using ALPS.
func Train(trainStmt *ir.TrainStmt, session *pb.Session) (string, error) {
	trainParams, validateParams, modelParams := tensorflow.CategorizeAttributes(trainStmt)
	featureColumnsCode, fieldDescs, err := tensorflow.DeriveFeatureColumnCodeAndFieldDescs(trainStmt)
	if err != nil {
		return "", err
	}

	filler := &trainFiller{
		DataSource:        session.DbConnStr,
		TrainSelect:       trainStmt.Select,
		ValidationSelect:  trainStmt.ValidationSelect,
		Estimator:         trainStmt.Estimator,
		FieldDescs:        fieldDescs,
		FeatureColumnCode: fmt.Sprintf("{%s}", strings.Join(featureColumnsCode, ",\n")),
		Y:                 trainStmt.Label.GetFieldDesc()[0],
		ModelParams:       modelParams,
		TrainParams:       trainParams,
		ValidationParams:  validateParams,
		Save:              trainStmt.Into,
		TmpTrainTable:     trainStmt.TmpTrainTable,
		TmpValidateTable:  trainStmt.TmpValidateTable,
	}

	var program bytes.Buffer
	var trainTemplate = template.Must(template.New("Train").Funcs(template.FuncMap{
		"intArrayToJSONString": ir.MarshalToJSONString,
		"attrToPythonValue":    ir.AttrToPythonValue,
		"DTypeToString":        ir.DTypeToString,
	}).Parse(templateTrain))
	if err := trainTemplate.Execute(&program, filler); err != nil {
		return "", err
	}
	return program.String(), nil
}
