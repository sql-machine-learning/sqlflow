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

package xgboost

import (
	"bytes"
	"text/template"
)

// DocGenInMarkdown generates the doc of the XGBoost in Markdown format.
func DocGenInMarkdown() string {
	var doc bytes.Buffer
	docTemplate.Execute(&doc, attributeDictionary.GenerateTableInHTML())

	return doc.String()
}

const docTemplateText = `# XGBoost Parameters

## TRAIN

### Example

` + "```SQL" + `
SELECT * FROM boston.train
TRAIN xgboost.gbtree
WITH
    objective="reg:squarederror",
    train.num_boost_round = 30
COLUMN crim, zn, indus, chas, nox, rm, age, dis, rad, tax, ptratio, b, lstat
LABEL medv
INTO sqlflow_models.my_xgb_regression_model;
` + "```" + `

### Parameters

{{.}}

## PREDICT

TBD

## ANALYZE

TBD
`

var docTemplate = template.Must(template.New("Doc").Parse(docTemplateText))
