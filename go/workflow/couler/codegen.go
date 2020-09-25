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

package couler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"

	"sqlflow.org/sqlflow/go/ir"
	pb "sqlflow.org/sqlflow/go/proto"
)

var defaultDockerImage = "sqlflow/sqlflow"
var workflowTTL = 24 * 3600
var envResource = "SQLFLOW_WORKFLOW_RESOURCES"

// Codegen generates Couler program
type Codegen struct{}

func fillMapIfValueNotEmpty(m map[string]string, key, value string) {
	if value != "" {
		m[key] = value
	}
}

func fillEnvFromSession(envs *map[string]string, session *pb.Session) {
	fillMapIfValueNotEmpty(*envs, "SQLFLOW_USER_TOKEN", session.Token)
	fillMapIfValueNotEmpty(*envs, "SQLFLOW_DATASOURCE", session.DbConnStr)
	fillMapIfValueNotEmpty(*envs, "SQLFLOW_USER_ID", session.UserId)
	fillMapIfValueNotEmpty(*envs, "SQLFLOW_HIVE_LOCATION", session.HiveLocation)
	fillMapIfValueNotEmpty(*envs, "SQLFLOW_HDFS_NAMENODE_ADDR", session.HdfsNamenodeAddr)
	fillMapIfValueNotEmpty(*envs, "SQLFLOW_HADOOP_USER", session.HdfsUser)
	fillMapIfValueNotEmpty(*envs, "SQLFLOW_HADOOP_PASS", session.HdfsUser)
	fillMapIfValueNotEmpty(*envs, "SQLFLOW_submitter", session.Submitter)
	fillMapIfValueNotEmpty(*envs, "SQLFLOW_WORKFLOW_STEP_LOG_FILE", os.Getenv("SQLFLOW_WORKFLOW_STEP_LOG_FILE"))
	fillMapIfValueNotEmpty(*envs, "SQLFLOW_WORKFLOW_EXIT_TIME_WAIT", os.Getenv("SQLFLOW_WORKFLOW_EXIT_TIME_WAIT"))
}

// GetStepEnvs returns a map of envs used for couler workflow.
func GetStepEnvs(session *pb.Session) (map[string]string, error) {
	envs := make(map[string]string)
	// fill step envs from the environment variables on sqlflowserver
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			return nil, fmt.Errorf("env: %s should format key=value", env)
		}
		// should not pass the workflow env into step
		if strings.HasPrefix(pair[0], "SQLFLOW_") && !strings.HasPrefix(pair[0], "SQLFLOW_WORKFLOW_") {
			envs[pair[0]] = pair[1]
		}
	}
	fillEnvFromSession(&envs, session)
	return envs, nil
}

// VerifyResources verifies the SQLFLOW_WORKFLOW_RESOURCES env to be valid.
func VerifyResources(resources string) error {
	if resources != "" {
		var r map[string]interface{}
		if e := json.Unmarshal([]byte(resources), &r); e != nil {
			return fmt.Errorf("%s: %s should be JSON format", envResource, resources)
		}
	}
	return nil
}

// GetSecret returns the workflow secret name, value.
func GetSecret() (string, string, error) {
	secretMap := make(map[string]map[string]string)
	secretCfg := os.Getenv("SQLFLOW_WORKFLOW_SECRET")
	if secretCfg == "" {
		return "", "", nil
	}
	if e := json.Unmarshal([]byte(secretCfg), &secretMap); e != nil {
		return "", "", e
	}
	if len(secretMap) != 1 {
		return "", "", fmt.Errorf(`SQLFLOW_WORKFLOW_SECRET should be a json string, e.g. {name: {key: value, ...}}`)
	}
	name := reflect.ValueOf(secretMap).MapKeys()[0].String()
	value, e := json.Marshal(secretMap[name])
	if e != nil {
		return "", "", e
	}
	return name, string(value), nil
}

// GenFiller generates Filler to fill the template
func GenFiller(programIR []ir.SQLFlowStmt, session *pb.Session) (*Filler, error) {
	stepEnvs, err := GetStepEnvs(session)
	if err != nil {
		return nil, err
	}
	if os.Getenv("SQLFLOW_WORKFLOW_TTL") != "" {
		workflowTTL, err = strconv.Atoi(os.Getenv("SQLFLOW_WORKFLOW_TTL"))
		if err != nil {
			return nil, fmt.Errorf("SQLFLOW_WORKFLOW_TTL: %s should be int", os.Getenv("SQLFLOW_WORKFLOW_TTL"))
		}
	}
	secretName, secretData, e := GetSecret()
	if e != nil {
		return nil, e
	}
	if e := VerifyResources(os.Getenv(envResource)); e != nil {
		return nil, e
	}

	stepLogFile := os.Getenv("SQLFLOW_WORKFLOW_STEP_LOG_FILE")

	r := &Filler{
		DataSource:  session.DbConnStr,
		StepEnvs:    stepEnvs,
		WorkflowTTL: workflowTTL,
		SecretName:  secretName,
		SecretData:  secretData,
		Resources:   os.Getenv(envResource),
		StepLogFile: stepLogFile,
	}
	// NOTE(yancey1989): does not use ModelImage here since the Predict statement
	// does not contain the ModelImage field in SQL Program IR.
	if os.Getenv("SQLFLOW_WORKFLOW_STEP_IMAGE") != "" {
		defaultDockerImage = os.Getenv("SQLFLOW_WORKFLOW_STEP_IMAGE")
	}

	for _, sqlIR := range programIR {
		switch i := sqlIR.(type) {
		case *ir.NormalStmt, *ir.PredictStmt, *ir.ExplainStmt, *ir.EvaluateStmt:
			// TODO(typhoonzero): get model image used when training.
			sqlStmt := &sqlStatement{
				OriginalSQL: sqlIR.GetOriginalSQL(), IsExtendedSQL: sqlIR.IsExtended(),
				DockerImage: defaultDockerImage}
			r.SQLStatements = append(r.SQLStatements, sqlStmt)
		case *ir.TrainStmt:
			stepImage := defaultDockerImage
			if i.ModelImage != "" {
				stepImage = i.ModelImage
			}
			if r.SQLFlowSubmitter == "katib" {
				sqlStmt, err := ParseKatibSQL(sqlIR.(*ir.TrainStmt))
				if err != nil {
					return nil, fmt.Errorf("Fail to parse Katib train statement %s", sqlIR.GetOriginalSQL())
				}
				r.SQLStatements = append(r.SQLStatements, sqlStmt)
			} else {
				sqlStmt := &sqlStatement{
					OriginalSQL: sqlIR.GetOriginalSQL(), IsExtendedSQL: sqlIR.IsExtended(),
					DockerImage: stepImage}
				r.SQLStatements = append(r.SQLStatements, sqlStmt)
			}
		case *ir.ShowTrainStmt, *ir.OptimizeStmt:
			sqlStmt := &sqlStatement{
				OriginalSQL:   sqlIR.GetOriginalSQL(),
				IsExtendedSQL: sqlIR.IsExtended(),
				DockerImage:   defaultDockerImage,
			}
			r.SQLStatements = append(r.SQLStatements, sqlStmt)
		case *ir.RunStmt:
			sqlStmt := &sqlStatement{
				OriginalSQL:   sqlIR.GetOriginalSQL(),
				IsExtendedSQL: sqlIR.IsExtended(),
				DockerImage:   i.ImageName,
				Select:        i.Select,
				Into:          i.Into}
			r.SQLStatements = append(r.SQLStatements, sqlStmt)
		default:
			return nil, fmt.Errorf("unrecognized IR type: %v", i)
		}
	}
	return r, nil
}

// GenCode generates a Couler program
func (cg *Codegen) GenCode(programIR []ir.SQLFlowStmt, session *pb.Session) (string, error) {
	r, e := GenFiller(programIR, session)
	if e != nil {
		return "", e
	}
	var program bytes.Buffer
	if err := coulerTemplate.Execute(&program, r); err != nil {
		return "", err
	}
	return program.String(), nil
}

// GenYAML translate the Couler program into Argo YAML
func (cg *Codegen) GenYAML(coulerProgram string) (string, error) {
	cmdline := bytes.Buffer{}
	fmt.Fprintf(&cmdline, "couler run --mode argo --workflow_name sqlflow ")
	if c := os.Getenv("SQLFLOW_WORKFLOW_CLUSTER_CONFIG"); len(c) > 0 {
		fmt.Fprintf(&cmdline, "--cluster_config %s ", c)
	}
	fmt.Fprintf(&cmdline, "--file -")

	coulerExec := strings.Split(cmdline.String(), " ")
	// execute command: `cat sqlflow.couler | couler run --mode argo --workflow_name sqlflow --file -`
	cmd := exec.Command(coulerExec[0], coulerExec[1:]...)
	cmd.Env = append(os.Environ())
	cmd.Stdin = strings.NewReader(coulerProgram)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed %s, %v %s", cmd, err, out)
	}
	return string(out), nil
}

// MockSQLProgramIR mock a SQLFLow program which contains multiple statements
func MockSQLProgramIR() []ir.SQLFlowStmt {
	normalStmt := ir.NormalStmt("SELECT * FROM iris.train limit 10;")
	trainStmt := ir.MockTrainStmt(true)
	return []ir.SQLFlowStmt{&normalStmt, trainStmt}
}
