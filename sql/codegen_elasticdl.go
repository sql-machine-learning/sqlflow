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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	pb "github.com/sql-machine-learning/sqlflow/server/proto"
)

var elasticdlTrainTemplate = template.Must(template.New("elasticdl_train").Parse(elasticdlTrainTemplateText))
var elasticdlDataConversionTemplate = template.Must(template.New("elasticdl_data_conversion").Parse(elasticdlDataConversionTemplateText))

type elasticDLDataConversionFiller struct {
	FeaturesList    string
	ODPSTableName   string
	RecordIODataDir string
	BatchSize       int
	NumProcesses    int
}

type elasticDLFiller struct {
	// Training or Predicting
	IsTraining bool

	// Input & Output
	TrainInputTable    string
	EvalInputTable     string
	PredictInputTable  string
	PredictOutputTable string
	PredictInputModel  string
	PredictOutputShape int

	FeaturesDescription string
	LabelColName        string

	TrainClause *resolvedTrainClause
}

func getFeaturesNames(pr *extendedSelect) ([]string, error) {
	selectFeatures := pr.standardSelect.fields.Strings()
	if len(selectFeatures) == 1 && selectFeatures[0] == "*" {
		log.Fatalf("ElasticDL doesn't support wildcard select yet")
	}
	features := make([]string, 0)
	for _, feature := range selectFeatures {
		if feature != pr.label {
			features = append(features, feature)
		}
	}
	return features, nil
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

func newElasticDLDataConversionFiller(pr *extendedSelect, batchSize int, numProcesses int) (*elasticDLDataConversionFiller, error) {
	recordIODataDir, err := ioutil.TempDir("/tmp", "recordio_data_dir_")
	if err != nil {
		return nil, err
	}
	featureNames, err := getFeaturesNames(pr)
	if err != nil {
		log.Fatalf("Failed to get feature names from SELECT statement %v", err)
		return nil, err
	}
	return &elasticDLDataConversionFiller{
		FeaturesList:    makePythonListCode(append(featureNames, pr.label)),
		ODPSTableName:   pr.tables[0],
		RecordIODataDir: recordIODataDir,
		BatchSize:       batchSize,
		NumProcesses:    numProcesses,
	}, err
}

func newElasticDLTrainFiller(pr *extendedSelect, db *DB, session *pb.Session, ds *trainAndValDataset) (*elasticDLFiller, error) {
	resolved, err := resolveTrainClause(&pr.trainClause)
	if err != nil {
		return nil, err
	}
	featureNames, err := getFeaturesNames(pr)
	if err != nil {
		log.Fatalf("Failed to get feature names from SELECT statement %v", err)
		return nil, err
	}
	var trainInput, evalInput string
	if ds != nil {
		trainInput, evalInput = ds.training, ds.validation
	} else {
		trainInput, evalInput = pr.tables[0], pr.tables[0]
	}
	return &elasticDLFiller{
		IsTraining:          true,
		TrainInputTable:     trainInput,
		EvalInputTable:      evalInput,
		FeaturesDescription: genFeaturesDescription(featureNames),
		LabelColName:        pr.label,
		TrainClause:         resolved,
	}, err
}

func newElasticDLPredictFiller(pr *extendedSelect, outputShape int) (*elasticDLFiller, error) {
	featureNames, err := getFeaturesNames(pr)
	if err != nil {
		log.Fatalf("Failed to get feature names from SELECT statement %v", err)
		return nil, err
	}
	return &elasticDLFiller{
		IsTraining:          false,
		PredictInputTable:   pr.tables[0],
		PredictOutputTable:  pr.predictClause.into,
		PredictInputModel:   pr.predictClause.model,
		PredictOutputShape:  outputShape,
		FeaturesDescription: genFeaturesDescription(featureNames),
	}, err
}

func elasticDLTrain(w *PipeWriter, pr *extendedSelect, db *DB, cwd string, session *pb.Session, ds *trainAndValDataset) error {
	// Write data conversion script
	// TODO: Execute the script inside container where ElasticDL is available
	var dataConversionProgram bytes.Buffer
	dataConversionFiller, err := newElasticDLDataConversionFiller(pr, 200, 1)
	if err != nil {
		return err
	}
	if err = elasticdlDataConversionTemplate.Execute(&dataConversionProgram, dataConversionFiller); err != nil {
		return fmt.Errorf("Failed executing data conversion template: %v", err)
	}
	dataConversionScriptPath := "data_conversion.py"
	dataConversionScript, err := os.Create(filepath.Join(cwd, dataConversionScriptPath))
	if err != nil {
		return fmt.Errorf("Create python code failed %v", err)
	}
	dataConversionScript.WriteString(dataConversionProgram.String())
	dataConversionScript.Close()

	// Write model definition file
	var elasticdlProgram bytes.Buffer
	trainFiller, err := newElasticDLTrainFiller(pr, db, session, ds)
	if err != nil {
		return err
	}

	if err = elasticdlTrainTemplate.Execute(&elasticdlProgram, trainFiller); err != nil {
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
	cmd := elasticdlCmd(cwd, "train")
	cmd.Args = append(cmd.Args, modelDefFilePath)
	cmd.Stdout = cw
	cmd.Stderr = cw
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("code %v failed %v", modelDefCode, e)
	}
	return nil
}

func elasticdlCmd(cwd, subCommand string) (cmd *exec.Cmd) {
	if hasDocker() {
		cmd = exec.Command("elasticdl", subCommand)
		cmd.Dir = cwd
	} else {
		log.Fatalf("Docker has to be installed to run ElasticDL command")
	}
	return cmd
}
