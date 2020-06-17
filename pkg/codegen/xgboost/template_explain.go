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
	"text/template"
)

type explainFiller struct {
	DataSource           string
	DatasetSQL           string
	ShapSummaryParams    string
	FeatureFieldMetaJSON string
	FeatureColumnNames   []string
	FeatureColumnCode    string
	LabelJSON            string
	IsPAI                bool
	PAIExplainTable      string
}

const explainTemplateText = `
import json
import sqlflow_submitter.xgboost as xgboost_extended
from sqlflow_submitter.xgboost.explain import explain

feature_field_meta = json.loads('''{{.FeatureFieldMetaJSON}}''')
label_spec = json.loads('''{{.LabelJSON}}''')
summary_params = json.loads('''{{.ShapSummaryParams}}''')

feature_column_names = [{{range .FeatureColumnNames}}
"{{.}}",
{{end}}]

transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(feature_column_names, {{.FeatureColumnCode}})

explain(
    datasource='''{{.DataSource}}''',
    select='''{{.DatasetSQL}}''',
	feature_field_meta=feature_field_meta,
	feature_column_names=feature_column_names,
    label_spec=label_spec,
    summary_params=summary_params,
    is_pai="{{.IsPAI}}" == "true",
    pai_explain_table="{{.PAIExplainTable}}",
    transform_fn=transform_fn,
    feature_column_code='''{{.FeatureColumnCode}}''')
`

var explainTemplate = template.Must(template.New("explain").Parse(explainTemplateText))
