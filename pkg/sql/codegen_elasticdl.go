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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	pb "sqlflow.org/sqlflow/pkg/server/proto"
)

var elasticdlModelDefTemplate = template.Must(template.New("elasticdl_train").Parse(elasticdlModelDefTemplateText))

type elasticDLFiller struct {
	// Training or Predicting
	IsTraining bool

	// Input & Output
	TrainInputTable    string
	EvalInputTable     string
	PredictInputTable  string
	PredictOutputTable string
	PredictInputModel  string
	OutputShape        int
	InputShape         int
	ModelDir           string

	FeaturesDescription string
	LabelColName        string
	FeaturesList        string

	TrainClause   *resolvedTrainClause
	PredictClause *resolvedPredictClause
}

type elasticDLModelSpec struct {
	NumClasses int
}

func getFeaturesNames(pr *extendedSelect, db *DB) ([]string, error) {
	fts, err := verify(pr, db)
	if err != nil {
		return nil, err
	}

	featureNames := make([]string, 0, len(fts))
	for featureName := range fts {
		if featureName != pr.label {
			featureNames = append(featureNames, featureName)
		}
	}
	sort.Strings(featureNames)
	return featureNames, nil
}

func genFeaturesDescription(featureNames []string) string {
	var sb strings.Builder
	for i, featureName := range featureNames {
		sb.WriteString(`"`)
		sb.WriteString(featureName)
		sb.WriteString(`"`)
		sb.WriteString(`: tf.io.FixedLenFeature([1], tf.float32),`)
		if i != len(featureNames)-1 {
			sb.WriteString(` `)
		}
	}
	return sb.String()
}

func makePythonListCode(items []string) string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, item := range items {
		sb.WriteString(`"`)
		sb.WriteString(item)
		sb.WriteString(`"`)
		if i != len(items)-1 {
			sb.WriteString(`, `)
		}
	}
	sb.WriteString("]")
	return sb.String()
}

func getElasticDLModelSpec(attrs map[string]*attribute) elasticDLModelSpec {
	getInt := func(key string, defaultValue int) int {
		if p, ok := attrs[key]; ok {
			strVal, _ := p.Value.(string)
			intVal, err := strconv.Atoi(strVal)

			if err == nil {
				return intVal
			}
		}
		return defaultValue
	}

	return elasticDLModelSpec{
		NumClasses: getInt("num_classes", 1),
	}
}

func newElasticDLTrainFiller(pr *extendedSelect, db *DB, session *pb.Session, ds *trainAndValDataset) (*elasticDLFiller, error) {
	resolved, err := resolveTrainClause(&pr.trainClause, &pr.standardSelect, nil)
	if err != nil {
		return nil, err
	}
	featureNames, err := getFeaturesNames(pr, db)
	if err != nil {
		log.Fatalf("Failed to get feature names from SELECT statement %v", err)
		return nil, err
	}
	hasFeatureColumns := false
	for _, columns := range resolved.FeatureColumns {
		if len(columns) > 0 {
			hasFeatureColumns = true
		}
	}
	if hasFeatureColumns {
		log.Warnln("COLUMN clause is ignored since ElasticDL does not support feature columns yet")
	}

	var trainInput, evalInput string
	if ds != nil {
		trainInput, evalInput = ds.training, ds.validation
	} else {
		trainInput, evalInput = pr.tables[0], pr.tables[0]
	}
	return &elasticDLFiller{
		IsTraining:      true,
		TrainInputTable: trainInput,
		EvalInputTable:  evalInput,
		FeaturesList:    makePythonListCode(append(featureNames, pr.label)),
		LabelColName:    pr.label,
		TrainClause:     resolved,
		ModelDir:        pr.trainClause.save,
		InputShape:      len(featureNames),
		OutputShape:     getElasticDLModelSpec(resolved.ModelConstructorParams).NumClasses,
	}, err
}

func newElasticDLPredictFiller(pr *extendedSelect, db *DB) (*elasticDLFiller, error) {
	resolved, err := resolvePredictClause(&pr.predictClause)
	if err != nil {
		return nil, err
	}
	featureNames, err := getFeaturesNames(pr, db)
	if err != nil {
		log.Fatalf("Failed to get feature names from SELECT statement %v", err)
		return nil, err
	}
	return &elasticDLFiller{
		IsTraining:         false,
		PredictInputTable:  pr.tables[0],
		PredictOutputTable: resolved.OutputTable,
		PredictInputModel:  resolved.ModelName,
		OutputShape:        getElasticDLModelSpec(resolved.ModelConstructorParams).NumClasses,
		FeaturesList:       makePythonListCode(append(featureNames, pr.label)),
		InputShape:         len(featureNames),
		PredictClause:      resolved,
	}, err
}

func elasticDLTrain(w *PipeWriter, pr *extendedSelect, db *DB, cwd string, session *pb.Session, ds *trainAndValDataset) error {
	// Write model definition file
	var elasticdlProgram bytes.Buffer
	trainFiller, err := newElasticDLTrainFiller(pr, db, session, ds)
	if err != nil {
		return err
	}

	if err = elasticdlModelDefTemplate.Execute(&elasticdlProgram, trainFiller); err != nil {
		return fmt.Errorf("Failed executing ElasticDL training template: %v", err)
	}
	modelDefCode := elasticdlProgram.String()
	cw := &logChanWriter{wr: w}
	modelDefFilePath := "model_definition.py"
	modelDefFile, err := os.Create(filepath.Join(cwd, modelDefFilePath))
	if err != nil {
		return fmt.Errorf("Create python code failed %v", err)
	}
	modelDefFile.WriteString(modelDefCode)
	modelDefFile.Close()

	// Create and execute ElasticDL training command
	cmd := elasticdlTrainCmd(cwd, modelDefFilePath, trainFiller)
	cmd.Stdout = cw
	cmd.Stderr = cw
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("code %v failed %v", modelDefCode, e)
	}
	return nil
}

func elasticdlTrainCmd(cwd, modelDefFilePath string, filler *elasticDLFiller) (cmd *exec.Cmd) {
	if hasDocker() {
		cmd = exec.Command(
			"elasticdl", "train",
			"--image_base", "elasticdl:ci",
			// TODO: Generate this dynamically
			"--job_name", "edl-sqlflow-train-job",
			// TODO: Get this from model name
			"--model_zoo", "/elasticdl/model_zoo",
			"--model_def", modelDefFilePath,
			"--training_data", filler.TrainInputTable,
			"--evaluation_data", filler.EvalInputTable,
			fmt.Sprintf("--num_epochs=%d", filler.TrainClause.Epoch),
			"--master_resource_request", filler.TrainClause.EngineParams.masterResourceRequest,
			"--master_resource_limit", filler.TrainClause.EngineParams.masterResourceLimit,
			"--worker_resource_request", filler.TrainClause.EngineParams.workerResourceRequest,
			"--worker_resource_limit", filler.TrainClause.EngineParams.workerResourceLimit,
			fmt.Sprintf("--num_workers=%d", filler.TrainClause.EngineParams.worker.Num),
			"--volume", filler.TrainClause.EngineParams.volume,
			"--image_pull_policy", filler.TrainClause.EngineParams.imagePullPolicy,
			"--restart_policy", filler.TrainClause.EngineParams.restartPolicy,
			"--extra_pypi_index", filler.TrainClause.EngineParams.extraPypiIndex,
			"--namespace", filler.TrainClause.EngineParams.namespace,
			fmt.Sprintf("--minibatch_size=%d", filler.TrainClause.EngineParams.minibatchSize),
			"--master_pod_priority", filler.TrainClause.EngineParams.masterPodPriority,
			"--cluster_spec", filler.TrainClause.EngineParams.clusterSpec,
			fmt.Sprintf("--num_minibatches_per_task=%d", filler.TrainClause.EngineParams.numMinibatchesPerTask),
			"--log_level", "INFO",
			"--output", filler.ModelDir,
			fmt.Sprintf("--checkpoint_steps=%d", filler.TrainClause.CheckpointSteps),
			fmt.Sprintf("--evaluation_steps=%d", filler.TrainClause.EvalSteps),
			fmt.Sprintf("--grads_to_wait=%d", filler.TrainClause.GradsToWait),
			"--tensorboard_log_dir", filler.TrainClause.TensorboardLogDir,
			"--checkpoint_dir", filler.TrainClause.CheckpointDir,
			fmt.Sprintf("--keep_checkpoint_max=%d", filler.TrainClause.KeepCheckpointMax),
			"--docker_image_repository", string(filler.TrainClause.EngineParams.dockerImageRepository),
			"--envs", string(filler.TrainClause.EngineParams.envs),
			"--data_reader_params", `'columns=`+string(filler.FeaturesList+`'`),
		)
		cmd.Dir = cwd
	} else {
		log.Fatalf("Docker has to be installed to run ElasticDL command")
	}
	return cmd
}

func elasticDLPredict(w *PipeWriter, pr *extendedSelect, db *DB, cwd string, session *pb.Session, ds *trainAndValDataset) error {
	// Write model definition file
	var elasticdlProgram bytes.Buffer
	predictFiller, err := newElasticDLPredictFiller(pr, db)
	if err != nil {
		return err
	}

	if err = elasticdlModelDefTemplate.Execute(&elasticdlProgram, predictFiller); err != nil {
		return fmt.Errorf("Failed executing ElasticDL prediction template: %v", err)
	}
	modelDefCode := elasticdlProgram.String()
	cw := &logChanWriter{wr: w}
	modelDefFilePath := "model_definition.py"
	modelDefFile, err := os.Create(filepath.Join(cwd, modelDefFilePath))
	if err != nil {
		return fmt.Errorf("Create python code failed %v", err)
	}
	modelDefFile.WriteString(modelDefCode)
	modelDefFile.Close()

	// Create and execute ElasticDL prediction command
	cmd := elasticdlPredictCmd(cwd, modelDefFilePath, predictFiller)
	cmd.Stdout = cw
	cmd.Stderr = cw
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("code %v failed %v", modelDefCode, e)
	}
	return nil
}

func elasticdlPredictCmd(cwd, modelDefFilePath string, filler *elasticDLFiller) (cmd *exec.Cmd) {
	if hasDocker() && hasElasticDLCmd() {
		cmd = exec.Command(
			"elasticdl", "predict",
			"--image_base", "elasticdl:ci",
			// TODO: Generate this dynamically
			"--job_name", "edl-sqlflow-predict-job",
			// TODO: Get this from model name
			"--model_zoo", "/elasticdl/model_zoo",
			"--model_def", modelDefFilePath,
			"--prediction_data", filler.PredictInputTable,
			"--checkpoint_filename_for_init", filler.PredictClause.CheckpointFilenameForInit,
			"--master_resource_request", filler.PredictClause.EngineParams.masterResourceRequest,
			"--master_resource_limit", filler.PredictClause.EngineParams.masterResourceLimit,
			"--worker_resource_request", filler.PredictClause.EngineParams.workerResourceRequest,
			"--worker_resource_limit", filler.PredictClause.EngineParams.workerResourceLimit,
			fmt.Sprintf("--num_workers=%d", filler.PredictClause.EngineParams.worker.Num),
			"--volume", filler.PredictClause.EngineParams.volume,
			"--image_pull_policy", filler.PredictClause.EngineParams.imagePullPolicy,
			"--restart_policy", filler.PredictClause.EngineParams.restartPolicy,
			"--extra_pypi_index", filler.PredictClause.EngineParams.extraPypiIndex,
			"--namespace", filler.PredictClause.EngineParams.namespace,
			fmt.Sprintf("--minibatch_size=%d", filler.PredictClause.EngineParams.minibatchSize),
			"--master_pod_priority", filler.PredictClause.EngineParams.masterPodPriority,
			"--cluster_spec", filler.PredictClause.EngineParams.clusterSpec,
			fmt.Sprintf("--num_minibatches_per_task=%d", filler.PredictClause.EngineParams.numMinibatchesPerTask),
			"--log_level", "INFO",
			"--docker_image_repository", string(filler.TrainClause.EngineParams.dockerImageRepository),
			"--envs", string(filler.TrainClause.EngineParams.envs),
			"--data_reader_params", `'columns=`+string(filler.FeaturesList+`'`),
		)
		cmd.Dir = cwd
	} else {
		log.Fatalf("Docker and ElasticDL CLI have to be installed to run ElasticDL")
	}
	return cmd
}
