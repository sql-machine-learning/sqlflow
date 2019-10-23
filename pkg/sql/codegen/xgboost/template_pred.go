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

type predFiller struct {
	DataSource      string
	PredSelect      string
	FeatureMetaJSON string
	LabelMetaJSON   string
	ResultTable     string
}

const predTemplateText = `
import json
import xgboost as xgb
import numpy as np
from sqlflow_submitter.db import connect_with_data_source, db_generator, buffered_db_writer

feature_field_meta = json.loads('''{{.FeatureMetaJSON}}''')
label_field_meta = json.loads('''{{.LabelMetaJSON}}''')

feature_column_names = [k["name"] for k in feature_field_meta]
label_name = label_field_meta["name"]

feature_specs = {k['name']: k for k in feature_field_meta}

conn = connect_with_data_source('''{{.DataSource}}''')

def xgb_dataset(fn, dataset_sql):
    gen = db_generator(conn.driver, conn, dataset_sql, feature_column_names, "", feature_specs)
    with open(fn, 'w') as f:
        for item in gen():
            features, label = item
            row_data = [str(label[0])] + ["%d:%f" % (i, v) for i, v in enumerate(features)]
            f.write("\t".join(row_data) + "\n")
    # TODO(yancey1989): genearte group and weight text file if necessary
    return xgb.DMatrix(fn)

dpred = xgb_dataset('predict.txt', """{{.PredSelect}}""")

bst = xgb.Booster({'nthread': 4})  # init model
bst.load_model("my_model")  # load data
preds = bst.predict(dpred)

# TODO(Yancey1989): using the train parameters to decide regressoin model or classifier model
if len(preds.shape) == 2:
    # classifier result
    preds = np.argmax(np.array(preds), axis=1)

feature_file_read = open("predict.txt", "r")

result_column_names = feature_column_names
result_column_names.append(label_name)
line_no = 0
with buffered_db_writer(conn.driver, conn, "{{.ResultTable}}", result_column_names, 100) as w:
    while True:
        line = feature_file_read.readline()
        if not line:
            break
        row = [i.split(":")[1] for i in line.replace("\n", "").split("\t")[1:]]
        row.append(str(preds[line_no]))
        w.write(row)
        line_no += 1
print("Done predicting. Predict table : {{.ResultTable}}")
`

var predTemplate = template.Must(template.New("Pred").Parse(predTemplateText))
