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

// Package ir is the Intermediate Representation of parsed SQL statements
package ir

// Executor is a visitor that generates and executes code for SQLFlowStmt
type Executor interface {
	ExecuteQuery(*NormalStmt) error
	ExecuteTrain(*TrainStmt) error
	ExecutePredict(*PredictStmt) error
	ExecuteExplain(*ExplainStmt) error
}

// SQLFlowStmt has multiple implementations: TrainStmt, PredictStmt, ExplainStmt and standard SQL.
type SQLFlowStmt interface {
	SetOriginalSQL(string)
	Execute(Executor) error
	IsExtended() bool
	GetOriginalSQL() string
}

// TrainStmt is the intermediate representation for code generation of a training job.
type TrainStmt struct {
	// OriginalSQL record the original SQL statement used to get current IR result
	// FIXME(typhoonzero): OriginalSQL is a temporary field. Can remove this when all moved to IR
	OriginalSQL string
	// Select specifies the query for fetching the training data. For example, "select * from iris.train;".
	Select string
	// ValidationSelect specifies the query for fetching the validation data. For example, "select * from iris.val;".
	ValidationSelect string
	// ModelImage is the name of the model's Docker image, for example `TO TRAIN a_data_scientist/regressors:v0.2/MyDNNRegressor`
	// the name "a_data_scientist/regressors:v0.2" is a Docker image.
	ModelImage string
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
	// Into specifies the table name in the INTO clause.
	Into string
	// When SQLFLOW_submitter == "pai", tmp tables will be created for training task
	// see: pai_submitter.go
	TmpTrainTable    string
	TmpValidateTable string
}

// Execute generates and executes code for TrainStmt
func (cl *TrainStmt) Execute(s Executor) error { return s.ExecuteTrain(cl) }

// SetOriginalSQL sets the original sql string
func (cl *TrainStmt) SetOriginalSQL(sql string) { cl.OriginalSQL = sql }

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (cl *TrainStmt) IsExtended() bool { return true }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (cl *TrainStmt) GetOriginalSQL() string { return cl.OriginalSQL }

// PredictStmt is the intermediate representation for code generation of a prediction job
//
// Please be aware the PredictStmt IR contains the result table name, so the
// generated Python program is responsible to create and write the result table.
type PredictStmt struct {
	// OriginalSQL record the original SQL statement used to get current IR result
	// FIXME(typhoonzero): OriginalSQL is a temporary field. Can remove this when all moved to IR
	OriginalSQL string
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
	// Using is the model specified by USING clause.
	Using string
	// TrainStmt is the TrainStmt used for generating the training job of the corresponding model
	TrainStmt *TrainStmt
	// When SQLFLOW_submitter == "pai", tmp tables will be created for predicting task
	// see: pai_submitter.go
	TmpPredictTable string
}

// Execute generates and executes code for PredictStmt
func (cl *PredictStmt) Execute(s Executor) error { return s.ExecutePredict(cl) }

// SetOriginalSQL sets the original sql string
func (cl *PredictStmt) SetOriginalSQL(sql string) { cl.OriginalSQL = sql }

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (cl *PredictStmt) IsExtended() bool { return true }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (cl *PredictStmt) GetOriginalSQL() string { return cl.OriginalSQL }

// ExplainStmt is the intermediate representation for code generation of a analysis job
type ExplainStmt struct {
	// OriginalSQL record the original SQL statement used to get current IR result
	// FIXME(typhoonzero): OriginalSQL is a temporary field. Can remove this when all moved to IR
	OriginalSQL string
	// Select specifies the query for fetching the analysis data. For example, "select * from iris.test;".
	Select string
	// Attributes is a map of parsed attribute in the WITH clause. For example, after parsing
	// "select ... explain ... with explain.plot_type = "bar"",
	// the Attributes will be {"explain.plot_type": "bar"}
	Attributes map[string]interface{}
	// Explainer types. For example TreeExplainer.
	Explainer string
	// ModelName is the model to be explained, e.g. TO EXPLAIN model_name
	ModelName string
	// Into stores the model explain result. Note that this field is optional.
	Into string
	// When SQLFLOW_submitter == "pai", tmp tables will be created for predicting task
	// see: pai_submitter.go
	TmpExplainTable string
	// TrainStmt is the TrainStmt used for generating the training job of the corresponding model
	TrainStmt *TrainStmt
}

// Execute generates and executes code for ExplainStmt
func (cl *ExplainStmt) Execute(s Executor) error { return s.ExecuteExplain(cl) }

// SetOriginalSQL sets the original sql string
func (cl *ExplainStmt) SetOriginalSQL(sql string) { cl.OriginalSQL = sql }

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (cl *ExplainStmt) IsExtended() bool { return true }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (cl *ExplainStmt) GetOriginalSQL() string { return cl.OriginalSQL }

// NormalStmt is a string of a standard SQL statement that can run on the database system.
type NormalStmt string

// Execute generates and executes code for NormalStmt
func (sql *NormalStmt) Execute(s Executor) error { return s.ExecuteQuery(sql) }

// SetOriginalSQL sets the original sql string
func (sql *NormalStmt) SetOriginalSQL(s string) {}

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (sql *NormalStmt) IsExtended() bool { return false }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (sql *NormalStmt) GetOriginalSQL() string { return string(*sql) }
