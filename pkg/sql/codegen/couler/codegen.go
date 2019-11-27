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

package couler

import (
	"bytes"

	"sqlflow.org/sqlflow/pkg/sql/ir"
)

// Run generates Couler program
func Run(programIR ir.SQLProgram) (string, error) {
	// TODO(yancey1989): fill session as env
	r := &coulerFiller{}
	for _, sqlIR := range programIR {
		ss := &sqlStatment{}
		switch sqlIR.(type) {
		case *ir.StandardSQL:
			ss.Extend = false
			ss.SQL = string(*sqlIR.(*ir.StandardSQL))
		case *ir.TrainClause:
			ss.Extend = true
			ss.SQL = sqlIR.(*ir.TrainClause).OriginalSQL
		case *ir.PredictClause:
			ss.Extend = true
			ss.SQL = sqlIR.(*ir.PredictClause).OriginalSQL
		case *ir.AnalyzeClause:
			ss.Extend = true
			ss.SQL = sqlIR.(*ir.AnalyzeClause).OriginalSQL
		}
		// TODO(yancey1989): using the custom Docker image in model zoo
		ss.DockerImage = "sqlflow/sqlflow"
		r.SQLStatements = append(r.SQLStatements, ss)
	}
	var program bytes.Buffer
	if err := coulerTemplate.Execute(&program, r); err != nil {
		return "", err
	}
	return program.String(), nil
}
