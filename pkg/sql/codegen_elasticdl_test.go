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
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "sqlflow.org/sqlflow/pkg/server/proto"
)

func TestTrainElasticDLFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	wndStatement := `SELECT * FROM iris.train
		TRAIN ElasticDLKerasClassifier 
		WITH
			model.optimizer = "optimizer",
			model.loss = "loss",
			model.eval_metrics_fn = "eval_metrics_fn",
			model.num_classes = 10,
			model.dataset_fn = "dataset_fn",
			train.shuffle = 120,
			train.epoch = 2,
			train.grads_to_wait = 2,
			train.tensorboard_log_dir = "",
			train.checkpoint_steps = 0,
			train.checkpoint_dir = "",
			train.keep_checkpoint_max = 0,
			eval.steps = 0,
			eval.start_delay_secs = 100,
			eval.throttle_secs = 0,
			eval.checkpoint_filename_for_init = "",
			engine.docker_image_prefix = "",
			engine.master_resource_request = "cpu=400m,memory=1024Mi",
			engine.master_resource_limit = "cpu=400m,memory=1024Mi",
			engine.worker_resource_request = "cpu=400m,memory=2048Mi",
			engine.worker_resource_limit = "cpu=1,memory=3072Mi",
			engine.num_workers = 2,
			engine.volume = "",
			engine.image_pull_policy = "Always",
			engine.restart_policy = "Never",
			engine.extra_pypi_index = "",
			engine.namespace = "default",
			engine.minibatch_size = 64,
			engine.master_pod_priority = "",
			engine.cluster_spec = "",
			engine.records_per_task = 100
		COLUMN
			sepal_length, sepal_width, petal_length, petal_width
		LABEL class
		INTO trained_elasticdl_keras_classifier;`

	r, e := parser.Parse(wndStatement)
	a.NoError(e)
	session := &pb.Session{UserId: "sqlflow_user"}
	filler, e := newElasticDLTrainFiller(r, testDB, session, nil)
	a.NoError(e)
	a.True(filler.IsTraining)
	a.Equal("iris.train", filler.TrainInputTable)
	a.Equal(true, filler.TrainClause.EnableShuffle)
	a.Equal(120, filler.TrainClause.ShuffleBufferSize)
	a.Equal("trained_elasticdl_keras_classifier", filler.ModelDir)

	var program bytes.Buffer
	e = elasticdlModelDefTemplate.Execute(&program, filler)
	a.NoError(e)
	code := program.String()
	a.True(strings.Contains(code, `if mode != Mode.PREDICTION and "true" == "true":`), code)
	a.True(strings.Contains(code, `dataset = dataset.shuffle(buffer_size=120)`), code)
	a.True(strings.Contains(code, `"class": tf.io.FixedLenFeature([1], tf.int64),`), code)
	a.True(strings.Contains(code, `"petal_length": tf.io.FixedLenFeature([1], tf.float32), "petal_width": tf.io.FixedLenFeature([1], tf.float32), "sepal_length": tf.io.FixedLenFeature([1], tf.float32), "sepal_width": tf.io.FixedLenFeature([1], tf.float32),`), code)
	a.True(strings.Contains(code, `labels = tf.cast(parsed_example["class"], tf.int64)`), code)
	a.True(strings.Contains(code, `return parsed_example, labels`), code)
	a.True(strings.Contains(code, `inputs = tf.keras.layers.Input(shape=(4, 1), name="input")`), code)
	a.True(strings.Contains(code, `outputs = tf.keras.layers.Dense(10, name="output")(inputs)`), code)
}

func TestPredElasticDLFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()
	predStatement := `SELECT sepal_length, sepal_width, petal_length, petal_width FROM iris.test
		PREDICT prediction_results_table
		WITH
			model.num_classes = 10
		USING trained_elasticdl_keras_classifier;`

	r, e := parser.Parse(predStatement)
	filler, err := newElasticDLPredictFiller(r, testDB)
	a.NoError(err)

	a.False(filler.IsTraining)
	a.Equal(filler.PredictInputTable, "iris.test")
	a.Equal(filler.PredictOutputTable, "prediction_results_table")
	a.Equal(filler.PredictInputModel, "trained_elasticdl_keras_classifier")
	a.Equal(filler.InputShape, 4)
	a.Equal(filler.OutputShape, 10)

	var program bytes.Buffer
	e = elasticdlModelDefTemplate.Execute(&program, filler)
	a.NoError(e)

	code := program.String()
	a.True(strings.Contains(code, `tf.keras.layers.Dense(10, name="output")(inputs)`), code)
	a.True(strings.Contains(code, `columns=["pred_" + str(i) for i in range(10)]`), code)
	a.True(strings.Contains(code, `column_types=["double" for _ in range(10)]`), code)
	a.True(strings.Contains(code, `table="prediction_results_table"`), code)
	a.True(strings.Contains(code, `"petal_length": tf.io.FixedLenFeature([1], tf.float32), "petal_width": tf.io.FixedLenFeature([1], tf.float32), "sepal_length": tf.io.FixedLenFeature([1], tf.float32), "sepal_width": tf.io.FixedLenFeature([1], tf.float32),`), code)
	a.True(strings.Contains(code, `inputs = tf.keras.layers.Input(shape=(4, 1), name="input")`), code)
}

func TestMakePythonListCode(t *testing.T) {
	a := assert.New(t)
	listCode := makePythonListCode([]string{"a", "b", "c"})
	a.Equal(`["a", "b", "c"]`, listCode)
}

func TestGenFeaturesDescription(t *testing.T) {
	a := assert.New(t)
	listCode := genFeaturesDescription([]string{"a", "b", "c"})
	a.Equal(`"a": tf.io.FixedLenFeature([1], tf.float32), "b": tf.io.FixedLenFeature([1], tf.float32), "c": tf.io.FixedLenFeature([1], tf.float32),`, listCode)
}
