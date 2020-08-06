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

package tensorflow

import (
	"bytes"
	"text/template"
)

// DocGenInMarkdown generates the doc of the XGBoost in Markdown format.
func DocGenInMarkdown() string {
	var doc bytes.Buffer
	docTemplate.Execute(&doc, commonAttributes.GenerateTableInHTML())

	return doc.String()
}

const docTemplateText = `# TensorFlow Parameters

## TRAIN

### Example

` + "```SQL" + `
SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH
    model.n_classes = 3, model.hidden_units = [10, 20],
    validation.select = "SELECT * FROM iris.test"
LABEL class
INTO sqlflow_models.my_dnn_model;
` + "```" + `

### Parameters

{{.}}

## PREDICT

TBD

## EXPLAIN

TBD
`

var docTemplate = template.Must(template.New("Doc").Parse(docTemplateText))
