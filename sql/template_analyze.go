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
	"text/template"
)

const analyzeTemplateText = `
import xgboost
import shap
import matplotlib.pyplot as plt
import pandas as pd

from sqlflow_submitter.db import connect, db_generator

shap.initjs()

X,y = shap.datasets.boston()

model = xgboost.train({"learning_rate": 0.01}, xgboost.DMatrix(X, label=y), 100)
explainer = shap.TreeExplainer(model)
shap_values = explainer.shap_values(X)

# summarize the effects of all the features
shap.summary_plot(shap_values, X, plot_type="dot")

plt.savefig('summary')

### vars 
# Session={{.Session}}

# read data
driver="{{.Driver}}"
feature_names = [{{ range $value := .Columns }} "{{$value}}", {{end}}]
label_name="{{.Label}}"
database=""
{{if ne .Database ""}}
database="{{.Database}}"
{{end}}
session_cfg = {}
{{ range $k, $v := .Session }}
session_cfg["{{$k}}"] = "{{$v}}"
{{end}}

conn = connect(driver, database, user="{{.User}}", password="{{.Password}}", host="{{.Host}}", port={{.Port}}, auth="{{.Auth}}")

# TODO(weiguo): load a model

`

var analyzeTemplate = template.Must(template.New("analyzeTemplate").Parse(analyzeTemplateText))
