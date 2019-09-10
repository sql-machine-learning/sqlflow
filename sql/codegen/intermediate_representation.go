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

package codegen

// FieldType indicates the field type of a table column
type FieldType int

const (
	// Int indicates the corresponding table column is an integer
	Int FieldType = iota
	// Float indicates the corresponding table column is a float
	Float
	// String indicates the corresponding table column is a string
	String
)

// FieldMeta contains the meta information for decoding. A field is a selected column of a SQL result.
//
// Name indicates the name for a field.
//
// DType indicates the data type for a field. For example: Int, Float, String.
//
// Delimiter indicates the decoding method of a field. For example, the field may
// contain a string like "1,23,42" which represent a 3-D tensor [1, 23, 42].
//
// Shape indicates the shape of the tensor represented for a field. For exmaple, the
// field may contain a string like "1,23,42" which represent a 3-D tensor, the shape
// will be [3].
//
// IsSparse indicates the type of tensor for a field. True means the tensor is a sparse tensor.
type FieldMeta struct {
	Name      string    `json:"name"`      // e.g. "spetal_length"
	DType     FieldType `json:"dtype"`     // e.g. "float", "int32"
	Delimiter string    `json:"delimiter"` // e.g. ","
	Shape     []int     `json:"shape"`     // e.g. [1], [1 2 3]
	IsSparse  bool      `json:"is_sparse"` // e.g. false
}

// FeatureColumn indicates the feature column to be applied on the field. Please refer to
// github.com/sql-machine-learning/sqlflow/sql/codegen/feature_column.go for detailed list of all feature columns.
type FeatureColumn interface{}

// TrainIR is the intermediate representation for code generation of a training job
type TrainIR struct {
	DataSource       string                     // e.g. "hive://root:root@localhost:10000/churn"
	Select           string                     // e.g. "select * from iris.train"
	ValidationSelect string                     // e.g. "select * from iris.val;"
	Estimator        string                     // e.g. "DNNClassifier"
	Attribute        map[string]interface{}     // e.g. {"train.epoch": 1000, "model.hidden_units": [10 10]}
	Feature          map[string][]FeatureColumn // e.g. {"feature_columns": {NumericColumn{...}}}
	Label            FeatureColumn              // e.g. NumericColumn{...}
}

// PredictIR is the intermediate representation for code generation of a prediction job
type PredictIR struct {
	DataSource  string                     // e.g. "hive://root:root@localhost:10000/churn"
	Select      string                     // e.g. "select * from iris.test"
	Estimator   string                     // e.g. "DNNClassifier"
	Attribute   map[string]interface{}     // e.g. {"predict.batch_size": 32}
	Feature     map[string][]FeatureColumn // e.g. {"feature_columns": {NumericColumn{...}}}
	Label       FeatureColumn              // e.g. NumericColumn{...}
	ResultTable string                     // e.g. "iris.predict"
}

// AnalyzeIR is the intermediate representation for code generation of a analysis job
type AnalyzeIR struct {
	DataSource string                     // e.g. "hive://root:root@localhost:10000/churn"
	Select     string                     // e.g. "select * from iris.train"
	Estimator  string                     // e.g. "DNNClassifier"
	Attribute  map[string]interface{}     // e.g. {"analyze.plot_type": "bar"}
	Feature    map[string][]FeatureColumn // e.g. {"feature_columns": {NumericColumn{...}}}
	Label      FeatureColumn              // e.g. NumericColumn{...}
}
