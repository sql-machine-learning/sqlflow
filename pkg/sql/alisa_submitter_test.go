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

package sql

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
)

func TestAlisaSubmitter(t *testing.T) {
	a := assert.New(t)
	_, ok := GetSubmitter("alisa").(*alisaSubmitter)
	a.True(ok)
}

func TestFindPyModulePath(t *testing.T) {
	a := assert.New(t)
	_, err := findPyModulePath("sqlflow_submitter")
	a.NoError(err)
}

func TestGetPAICmd(t *testing.T) {
	a := assert.New(t)
	cc := &pai.ClusterConfig{
		Worker: pai.WorkerConfig{
			Count: 1,
			CPU:   2,
			GPU:   0,
		},
		PS: pai.PSConfig{
			Count: 2,
			CPU:   4,
			GPU:   0,
		},
	}
	os.Setenv("SQLFLOW_OSS_CHECKPOINT_DIR", "oss://bucket/?role_arn=xxx&host=xxx")
	defer os.Unsetenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	paiCmd, err := getPAIcmd(cc, "my_model", "testdb.test", "", "testdb.result")
	a.NoError(err)
	ckpDir, err := pai.FormatCkptDir("my_model")
	a.NoError(err)
	expected := fmt.Sprintf("pai -name tensorflow1120 -DjobName=sqlflow_my_model -Dtags=dnn -Dscript=file://@@task.tar.gz -DentryFile=entry.py -Dtables=odps://testdb/tables/test -Doutputs=odps://testdb/tables/result -DcheckpointDir=\"%s\"", ckpDir)
	a.Equal(expected, paiCmd)
}
