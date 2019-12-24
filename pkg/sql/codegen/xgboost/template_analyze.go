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
	"text/template"
)

type analyzeFiller struct {
	DataSource        string
	DatasetSQL        string
	ShapSummaryParams string
	FieldDescJSON     string
	Label             string
}

const analyzeTemplateText = `
import xgboost
import shap
import json
import matplotlib
import matplotlib.pyplot as plt
import pandas as pd

from sqlflow_submitter.db import connect_with_data_source, db_generator

shap.initjs()

feature_field_meta = json.loads('''{{.FieldDescJSON}}''')
feature_column_name = sorted([k["name"] for k in feature_field_meta])
feature_spec = {k['name']: k for k in feature_field_meta}
conn = connect_with_data_source('''{{.DataSource}}''')
label_name = "{{.Label}}"

summaryAttrs = json.loads('''{{.ShapSummaryParams}}''')

def analyzer_dataset():
    stream = db_generator(conn.driver, conn, """{{.DatasetSQL}}""", feature_column_name, label_name, feature_spec)
    xs = pd.DataFrame(columns=feature_column_name)
    ys = pd.DataFrame(columns=[label_name])
    i = 0
    for row in stream():
        xs.loc[i] = row[0]
        ys.loc[i] = row[1]
        i += 1
    return xs, ys

X,y = analyzer_dataset()
bst = xgboost.Booster()
bst.load_model("my_model")
explainer = shap.TreeExplainer(bst)
shap_values = explainer.shap_values(X)
shap.summary_plot(shap_values, X, show=False, **summaryAttrs)
plt.savefig('summary', bbox_inches='tight')

matplotlib.use('module://plotille_backend')
import matplotlib.pyplot as plt
shap.summary_plot(shap_values, X, show=False, **summaryAttrs)
import sys
sys.stdout.isatty = lambda:True
plt.savefig('summary', bbox_inches='tight')
`

var analyzeTemplate = template.Must(template.New("analyze").Parse(analyzeTemplateText))
