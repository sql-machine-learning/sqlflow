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

package ir

// MockTrainStmt generates a sample TrainStmt for test.
func MockTrainStmt(datasource string, isxgboost bool) *TrainStmt {
	// SELECT * FROM iris_train TO TRAIN DNNClassifier
	// WITH train.batch_size=4, train.epoch=3, model.hidden_units=[10,20], model.n_classes=3
	// LABEL clas INTO my_dnn_model;
	attrs := map[string]interface{}{}
	estimator := "DNNClassifier"
	if isxgboost {
		attrs["train.num_boost_round"] = 10
		attrs["objective"] = "multi:softprob"
		attrs["eta"] = float32(0.1)
		attrs["num_class"] = 3
		estimator = "xgboost.gbtree"
	} else {
		attrs["train.batch_size"] = 4
		attrs["train.epoch"] = 3
		attrs["model.hidden_units"] = []int{10, 20}
		attrs["model.n_classes"] = 3
	}
	return &TrainStmt{
		DataSource:       datasource,
		Select:           "select * from iris.train;",
		ValidationSelect: "select * from iris.test;",
		Estimator:        estimator,
		Attributes:       attrs,
		Features: map[string][]FeatureColumn{
			"feature_columns": {
				&NumericColumn{&FieldDesc{"sepal_length", Float, "", []int{1}, false, nil, 0}},
				&NumericColumn{&FieldDesc{"sepal_width", Float, "", []int{1}, false, nil, 0}},
				&NumericColumn{&FieldDesc{"petal_length", Float, "", []int{1}, false, nil, 0}},
				&NumericColumn{&FieldDesc{"petal_width", Float, "", []int{1}, false, nil, 0}}}},
		Label: &NumericColumn{&FieldDesc{"class", Int, "", []int{1}, false, nil, 0}}}
}

// MockPredStmt generates a sample PredictStmt for test.
func MockPredStmt(trainStmt *TrainStmt) *PredictStmt {
	return &PredictStmt{
		DataSource:  trainStmt.DataSource,
		Select:      "select * from iris.test;",
		ResultTable: "iris.predict",
		Attributes:  make(map[string]interface{}),
		TrainStmt:   trainStmt,
	}
}
