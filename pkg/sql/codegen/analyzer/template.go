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

package analyzer

type filler struct {
	DataSource         string
	DatasetSQL         string
	X                  []*FeatureMeta
	Label              string
	ShapSummaryParames map[string]interface{}
	ModelFile          string
}

const templateText = `
import xgboost
import shap
import json 
import matplotlib.pyplot as plt
import pandas as pd

from sqlflow_submitter.db import connect_with_data_source, db_generator

shap.initjs()

feature_names = [{{ range $value := .X }} "{{$value.FeatureName}}", {{end}}]
feature_metas = {}
{{ range $value := .X }}
feature_metas["{{$value.FeatureName}}"] = {
    "feature_name": "{{$value.FeatureName}}",
    "dtype": "{{$value.Dtype}}",
    "delimiter": "{{$value.Delimiter}}",
    "shape": {{$value.InputShape}},
    "is_sparse": "{{$value.IsSparse}}" == "true"
}
{{end}}
conn = connect_with_data_source('''{{.DataSource}}''')
label_name = "{{.Label}}"
model_path = "{{.ModelFile}}"

summaryAttrs = {}
{{ range $k, $v := .ShapSummaryParames }}
summaryAttrs["{{$k}}"] = {{$v}}
{{end}}

def analyzer_dataset():
    stream = db_generator(conn.driver, conn, """{{.DatasetSQL}}""", feature_names, label_name, feature_metas)
    xs = pd.DataFrame(columns=feature_names)
    ys = pd.DataFrame(columns=[label_name])
    i = 0
    for row in stream():
        xs.loc[i] = row[0]
        ys.loc[i] = row[1]
        i += 1
    return xs, ys

X,y = analyzer_dataset()
bst = xgboost.Booster()
bst.load_model(fname=model_path)
explainer = shap.TreeExplainer(bst)
shap_values = explainer.shap_values(X)
shap.summary_plot(shap_values, X, show=False, **summaryAttrs)
plt.savefig('summary', bbox_inches='tight')
`

var template = template.Must(template.New("analyze").Parse(templateText))
