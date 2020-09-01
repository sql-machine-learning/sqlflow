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

import (
	"strings"
)

// SQLFlowStmt has multiple implementations: TrainStmt, PredictStmt, ExplainStmt and standard SQL.
type SQLFlowStmt interface {
	SetOriginalSQL(string)
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
	// PreTrainedModel specifies the model name to be loaded for incremental training.
	PreTrainedModel string
	// Into specifies the table name in the INTO clause.
	Into string
	// When SQLFLOW_submitter == "pai", tmp tables will be created for training task
	// see: pai_submitter.go
	TmpTrainTable    string
	TmpValidateTable string
}

const (
	// TensorFlow is a kind of `TrainStmt`
	TensorFlow = iota
	// XGBoost is a kind of `TrainStmt`
	XGBoost
	// KMeans is a kind of `TrainStmt`
	KMeans
)

// GetModelKind returns the kind of model in the TrainStmt
func (stmt *TrainStmt) GetModelKind() int {
	estimator := strings.ToUpper(stmt.Estimator)
	if strings.HasPrefix(estimator, "XGB") {
		return XGBoost
	}
	if strings.HasPrefix(estimator, "KMEANS") {
		return KMeans
	}
	return TensorFlow
}

// SetOriginalSQL sets the original sql string
func (stmt *TrainStmt) SetOriginalSQL(sql string) { stmt.OriginalSQL = sql }

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (stmt *TrainStmt) IsExtended() bool { return true }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (stmt *TrainStmt) GetOriginalSQL() string { return stmt.OriginalSQL }

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

// SetOriginalSQL sets the original sql string
func (stmt *PredictStmt) SetOriginalSQL(sql string) { stmt.OriginalSQL = sql }

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (stmt *PredictStmt) IsExtended() bool { return true }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (stmt *PredictStmt) GetOriginalSQL() string { return stmt.OriginalSQL }

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

// SetOriginalSQL sets the original sql string
func (stmt *ExplainStmt) SetOriginalSQL(sql string) { stmt.OriginalSQL = sql }

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (stmt *ExplainStmt) IsExtended() bool { return true }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (stmt *ExplainStmt) GetOriginalSQL() string { return stmt.OriginalSQL }

// EvaluateStmt is the intermediate representation for code generation of an evaluation job
type EvaluateStmt struct {
	OriginalSQL      string
	Select           string
	Attributes       map[string]interface{}
	ModelName        string
	Label            FeatureColumn
	Into             string
	TmpEvaluateTable string
	TrainStmt        *TrainStmt
}

// SetOriginalSQL sets the original sql string
func (stmt *EvaluateStmt) SetOriginalSQL(sql string) { stmt.OriginalSQL = sql }

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (stmt *EvaluateStmt) IsExtended() bool { return true }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (stmt *EvaluateStmt) GetOriginalSQL() string { return stmt.OriginalSQL }

// NormalStmt is a SQL statement without using SQLFlow syntax extension.
type NormalStmt string

// SetOriginalSQL sets the original sql string
func (stmt *NormalStmt) SetOriginalSQL(sql string) {}

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (stmt *NormalStmt) IsExtended() bool { return false }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (stmt *NormalStmt) GetOriginalSQL() string { return string(*stmt) }

// ShowTrainStmt get and output the original train sql for ModelName
type ShowTrainStmt struct {
	// OriginalSQL is the SHOW TRAIN stmt itself
	OriginalSQL string
	// The model to show the train sql
	ModelName string
}

// SetOriginalSQL sets the original sql string
func (stmt *ShowTrainStmt) SetOriginalSQL(sql string) { stmt.OriginalSQL = sql }

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (stmt *ShowTrainStmt) IsExtended() bool { return true }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (stmt *ShowTrainStmt) GetOriginalSQL() string { return stmt.OriginalSQL }

// OptimizeExpr is the intermediate code for generating target solver expressions.
type OptimizeExpr struct {
	// Objective expression or constraint expression string tokens prepared for generate target code.
	ExpressionTokens []string `json:"tokens"`
	// constraint group by like: SUM(markets) <= capacity GROUP BY plants, will expand to
	// for p in plants:
	//     sum(m for m in markets) <= capacity
	GroupBy string `json:"group_by"`
}

// OptimizeStmt is the intermediate representation of "SELECT TO MAXIMIZE|MINIMIZE" statement.
type OptimizeStmt struct {
	// OriginalSQL records the original SQL statement used to get current IR result
	OriginalSQL string
	// Select is the select statement before TO MAXIMIZE|MINIMIZE clause.
	Select string
	// Variables is the variable name list to be optimized
	Variables []string
	// ResultValueName is the column name of the result variable
	ResultValueName string
	// VariableType
	VariableType string
	// Attributes is a map of parsed attribute in the WITH clause.
	Attributes map[string]interface{}
	// Objective
	Objective OptimizeExpr
	// Direction, "maximize" or "minimize"
	Direction string
	// Constraints
	Constraints []*OptimizeExpr
	// Solver
	Solver string
	// ResultTable is the table name to store results.
	ResultTable string
}

// SetOriginalSQL sets the original sql string
func (stmt *OptimizeStmt) SetOriginalSQL(sql string) { stmt.OriginalSQL = sql }

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (stmt *OptimizeStmt) IsExtended() bool { return true }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (stmt *OptimizeStmt) GetOriginalSQL() string { return stmt.OriginalSQL }

// RunStmt is the intermediate representation of `SELECT TO RUN` statement
type RunStmt struct {
	// OriginalSQL is the `SELECT TO RUN` statement.
	OriginalSQL string
	// Select is the select statement before TO RUN clause.
	Select string
	// ImageName is the name of the docker image after TO RUN keyword.
	ImageName string
	// Parameters is the command line parameters for the docker image.
	Parameters []string
	// Into is the output table names (0~N, comma separated) after INTO keyword.
	Into string
}

// SetOriginalSQL sets the original sql string
func (stmt *RunStmt) SetOriginalSQL(sql string) { stmt.OriginalSQL = sql }

// GetOriginalSQL returns the original SQL statement used to get current IR result
func (stmt *RunStmt) GetOriginalSQL() string { return stmt.OriginalSQL }

// IsExtended returns whether a SQLFlowStmt is an extended SQL statement
func (stmt *RunStmt) IsExtended() bool { return true }
