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
// Shape indicates the shape of the tensor represented for a field. For example, the
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
	// Vocabulary stores all possible enumerate values if the column type is string,
	// e.g. the column values are: "MALE", "FEMALE", "NULL"
	Vocabulary map[string]string `json:"vocabulary"` // use a map to generate a list without duplication
	// if the column data is used as embedding(category_column()), the `num_buckets` should use the maxID
	// appeared in the sample data. if error still occurs, users should set `num_buckts` manually.
	MaxID int64
}

// FeatureColumn indicates the feature column to be applied on the field. Please refer to
// sqlflow.org/sqlflow/pkg/sql/codegen/feature_column.go for detailed list of all feature columns.
type FeatureColumn interface {
	GetFieldMeta() []*FieldMeta
}

// SQLFlowIR represent all kind of IRs including: TrainIR, PredictIR, AnalyzeIR and standard SQL.
type SQLFlowIR interface {
	// This function is used only for restrict the IR struct types
	IsIR()
}

// TrainIR is the intermediate representation for code generation of a training job.
//
// Please be aware that the TrainIR intentionally excludes the model table name in the
// INTO clause. The sql package will save the output files of a generated Python program.
// For prediction and analysis jobs, the sql will restore an identical working directly.
type TrainIR struct {
	// DataSource contains the connection information. For example, "hive://root:root@localhost:10000/churn"
	DataSource string
	// Select specifies the query for fetching the training data. For example, "select * from iris.train;".
	Select string
	// ValidationSelect specifies the query for fetching the validation data. For example, "select * from iris.val;".
	ValidationSelect string
	// Estimator specifies the estimator type. For example, after parsing "select ... train DNNClassifier WITH ...",
	// the Estimator will be "DNNClassifier".
	Estimator string
	// Attributes is a map of parsed attribute in the WITH Clause. For example, after parsing
	// "select ... train ... with train.epoch = 1000, model.hidden_units = [10, 10]",
	// the Attributes will be {"train.epoch": 1000, "model.hidden_units": [10 10]}.
	Attributes map[string]interface{}
	// Features contain a map of a list of feature columns in the COLUMN clause.
	// For multiple COLUMN clauses like
	//   ```
	//   column ... for deep_feature
	//   column ... for wide_feature
	//   ```
	// They will be parsed as {"deep_feature": {...}, "wide_feature": {...}}
	// For single column clause like "column ...", "feature_columns" will be used as the default map key.
	Features map[string][]FeatureColumn
	// Label specifies the feature column in the LABEL clause.
	Label FeatureColumn
}

// IsIR is used only for restrict the IR struct types
func (trainIR *TrainIR) IsIR() {}

// PredictIR is the intermediate representation for code generation of a prediction job
//
// Please be aware the PredictionIR contains the result table name, so the
// generated Python program is responsible to create and write the result table.
type PredictIR struct {
	// DataSource contains the connection information. For example, "hive://root:root@localhost:10000/churn"
	DataSource string
	// Select specifies the query for fetching the prediction data. For example, "select * from iris.test;".
	Select string
	// ResultTable specifies the table to store the prediction result.
	ResultTable string
	// ResultColumn is the column to store predict result in ResultTable
	ResultColumn string
	// Attributes is a map of parsed attribute in the WITH clause. For example, after parsing
	// "select ... predict ... with predict.batch_size = 32 into ...",
	// the Attributes will be {"predict.batch_size": 32}
	Attributes map[string]interface{}
	// TrainIR is the TrainIR used for generating the training job of the corresponding model
	TrainIR *TrainIR
}

// IsIR is used only for restrict the IR struct types
func (predictIR *PredictIR) IsIR() {}

// AnalyzeIR is the intermediate representation for code generation of a analysis job
type AnalyzeIR struct {
	// DataSource contains the connection information. For example, "hive://root:root@localhost:10000/churn"
	DataSource string
	// Select specifies the query for fetching the analysis data. For example, "select * from iris.test;".
	Select string
	// Attributes is a map of parsed attribute in the WITH clause. For example, after parsing
	// "select ... analyze ... with analyze.plot_type = "bar"",
	// the Attributes will be {"analyze.plot_type": "bar"}
	Attributes map[string]interface{}
	// Explainer types. For example TreeExplainer.
	Explainer string
	// TrainIR is the TrainIR used for generating the training job of the corresponding model
	TrainIR *TrainIR
}

// IsIR is used only for restrict the IR struct types
func (analyzeIR *AnalyzeIR) IsIR() {}

// StandardSQLIR is a string of a standard SQL statement that can run on the database system.
type StandardSQLIR string

// IsIR is used only for restrict the IR struct types
func (sql *StandardSQLIR) IsIR() {}
