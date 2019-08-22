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
	"io/ioutil"
	"strings"
	"testing"

	pb "github.com/sql-machine-learning/sqlflow/server/proto"
	"github.com/stretchr/testify/assert"
)

func TestTrainElasticDLFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	wndStatement := `SELECT c1, c2, c3, c4, c5 FROM training_data
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
			predict.checkpoint_filename_for_init = "",
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
			c1,
			NUMERIC(c2, 10),
			c3,
			c4
		LABEL c5
		INTO trained_elasticdl_keras_classifier;`

	r, e := parser.Parse(wndStatement)
	a.NoError(e)
	session := &pb.Session{UserId: "sqlflow_user"}
	filler, e := newElasticDLTrainFiller(r, nil, session, nil)
	a.NoError(e)
	a.True(filler.IsTraining)
	a.Equal("training_data", filler.TrainInputTable)
	a.Equal(true, filler.TrainClause.EnableShuffle)
	a.Equal(120, filler.TrainClause.ShuffleBufferSize)
	a.Equal("trained_elasticdl_keras_classifier", filler.ModelDir)

	var program bytes.Buffer
	e = elasticdlTrainTemplate.Execute(&program, filler)
	a.NoError(e)
	code := program.String()
	a.True(strings.Contains(code, `if mode != Mode.PREDICTION and "true" == "true":`), code)
	a.True(strings.Contains(code, `dataset = dataset.shuffle(buffer_size=120)`), code)
	a.True(strings.Contains(code, `"c5": tf.io.FixedLenFeature([1], tf.int64),`), code)
	a.True(strings.Contains(code, `"c1": tf.io.FixedLenFeature([1], tf.float32), "c2": tf.io.FixedLenFeature([1], tf.float32), "c3": tf.io.FixedLenFeature([1], tf.float32), "c4": tf.io.FixedLenFeature([1], tf.float32),`), code)
	a.True(strings.Contains(code, `return parsed_example, tf.cast(parsed_example["c5"], tf.int32)`), code)
}

func TestPredElasticDLFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()
	predStatement := `SELECT c1, c2, c3, c4 FROM prediction_data
		PREDICT prediction_results_table
		USING trained_elasticdl_keras_classifier;`

	r, e := parser.Parse(predStatement)
	filler, err := newElasticDLPredictFiller(r, 10)
	a.NoError(err)

	a.False(filler.IsTraining)
	a.Equal(filler.PredictInputTable, "prediction_data")
	a.Equal(filler.PredictOutputTable, "prediction_results_table")
	a.Equal(filler.PredictInputModel, "trained_elasticdl_keras_classifier")

	var program bytes.Buffer
	e = elasticdlTrainTemplate.Execute(&program, filler)
	a.NoError(e)

	code := program.String()
	a.True(strings.Contains(code, `tf.keras.layers.Dense(10, name="output")(flatten)`), code)
	a.True(strings.Contains(code, `columns=["pred_" + str(i) for i in range(10)]`), code)
	a.True(strings.Contains(code, `column_types=["double" for _ in range(10)]`), code)
	a.True(strings.Contains(code, `table = "prediction_results_table"`), code)
	a.True(strings.Contains(code, `"c1": tf.io.FixedLenFeature([1], tf.float32), "c2": tf.io.FixedLenFeature([1], tf.float32), "c3": tf.io.FixedLenFeature([1], tf.float32), "c4": tf.io.FixedLenFeature([1], tf.float32),`), code)
}

func TestElasticDLDataConversionFiller(t *testing.T) {
	a := assert.New(t)
	parser := newParser()

	wndStatement := `SELECT c1, c2, c3, c4, c5 FROM training_data
		TRAIN ElasticDLKerasClassifier 
		WITH
			model.optimizer = "optimizer",
			model.loss = "loss"
		COLUMN
			c1,
			NUMERIC(c2, 10),
			c3,
			c4
		LABEL c5
		INTO trained_elasticdl_keras_classifier;`

	r, e := parser.Parse(wndStatement)
	a.NoError(e)

	var program bytes.Buffer
	recordIODataDir, e := ioutil.TempDir("/tmp", "recordio_data_dir_")
	a.NoError(e)
	filler, e := newElasticDLDataConversionFiller(r, recordIODataDir, 200, 1)
	a.NoError(e)
	e = elasticdlDataConversionTemplate.Execute(&program, filler)
	a.NoError(e)
	code := program.String()
	a.True(strings.Contains(code, `table = "training_data"`), code)
	a.True(strings.Contains(code, `COLUMN_NAMES = ["c1", "c2", "c3", "c4", "c5"]`), code)
	a.True(strings.Contains(code, `output_dir = "/tmp/recordio_data_dir_`), code)
	a.True(strings.Contains(code, `batch_size = 200`), code)
	a.True(strings.Contains(code, `num_processes = 1`), code)
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
