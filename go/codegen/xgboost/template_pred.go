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

type predFiller struct {
	DataSource         string
	PredSelect         string
	FeatureMetaJSON    string
	TrainLabelMetaJSON string
	PredLabelMetaJSON  string
	FeatureColumnNames []string
	FeatureColumnCode  string
	ResultTable        string
	HDFSNameNodeAddr   string
	HiveLocation       string
	HDFSUser           string
	HDFSPass           string
	IsPAI              bool
	PAITable           string
}

const predTemplateText = `
import json
import runtime.xgboost as xgboost_extended
from runtime.xgboost.predict import pred

feature_metas = json.loads('''{{.FeatureMetaJSON}}''')
train_label_meta = json.loads('''{{.TrainLabelMetaJSON}}''')
pred_label_meta = json.loads('''{{.PredLabelMetaJSON}}''')

feature_column_names = [{{range .FeatureColumnNames}}
"{{.}}",
{{end}}]

feature_column_list = [{{.FeatureColumnCode}}]
transform_fn = xgboost_extended.feature_column.ComposedColumnTransformer(feature_column_names, *feature_column_list)

pred(datasource='''{{.DataSource}}''',
     select='''{{.PredSelect}}''',
     feature_metas=feature_metas,
     feature_column_names=feature_column_names,
     train_label_meta=train_label_meta,
     pred_label_meta=pred_label_meta,
     result_table='''{{.ResultTable}}''',
     is_pai="{{.IsPAI}}" == "true",
     pai_table="{{.PAITable}}",
     transform_fn=transform_fn,
     feature_column_code='''{{.FeatureColumnCode}}''')
`

var predTemplate = template.Must(template.New("Pred").Parse(predTemplateText))
