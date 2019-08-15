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
}

func TestElasticDLDataConversionFiller(t *testing.T) {
	a := assert.New(t)
	var program bytes.Buffer
	filler, e := newElasticDLDataConversionFiller("table_name", `["a", "b", "c"]`)
	a.NoError(e)
	e = elasticdlDataConversionTemplate.Execute(&program, filler)
	a.NoError(e)
	code := program.String()
	a.True(strings.Contains(code, `table = "table_name"`), code)
	a.True(strings.Contains(code, `COLUMN_NAMES = ["a", "b", "c"]`), code)
	a.True(strings.Contains(code, `output_dir = "/tmp/recordio_data_dir_`), code)
}
