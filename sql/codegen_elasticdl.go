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
	TrainInputTable   string
	EvalInputTable    string
	PredictInputTable string

	TrainClause *resolvedTrainClause
}

func newElasticDLDataConversionFiller(odpsTableName string, featuresList string, batchSize int, numProcesses int) (*elasticDLDataConversionFiller, error) {
	recordIODataDir, err := ioutil.TempDir("/tmp", "recordio_data_dir_")
	if err != nil {
		return nil, err
	}
	return &elasticDLDataConversionFiller{
		FeaturesList:    featuresList,
		ODPSTableName:   odpsTableName,
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
	var trainInput, evalInput string
	if ds != nil && ds.supported {
		trainInput, evalInput = ds.training, ds.validation
	} else {
		trainInput, evalInput = pr.tables[0], pr.tables[0]
	}
	return &elasticDLFiller{
		IsTraining:      true,
		TrainInputTable: trainInput,
		EvalInputTable:  evalInput,
		TrainClause:     resolved,
	}, err
}

func elasticDLTrain(w *PipeWriter, pr *extendedSelect, db *DB, cwd string, session *pb.Session, ds *trainAndValDataset) error {
	var program bytes.Buffer
	trainFiller, err := newElasticDLTrainFiller(pr, db, session, ds)
	if err != nil {
		return err
	}

	if err = elasticdlTrainTemplate.Execute(&program, trainFiller); err != nil {
		return fmt.Errorf("submitElasticDL: failed executing template: %v", err)
	}
	code := program.String()
	cw := &logChanWriter{wr: w}
	cmd := elasticdlCmd(cwd, "train")
	filename := "model_definition.py"
	absfile := filepath.Join(cwd, filename)
	f, err := os.Create(absfile)
	if err != nil {
		return fmt.Errorf("Create python code failed %v", err)
	}
	f.WriteString(program.String())
	f.Close()

	cmd.Args = append(cmd.Args, filename)
	cmd.Stdout = cw
	cmd.Stderr = cw
	if e := cmd.Run(); e != nil {
		return fmt.Errorf("code %v failed %v", code, e)
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
