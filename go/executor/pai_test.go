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

package executor

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A pai job may contains multiple log view URL for each Maxcompute job
const mockPAIOutput = `
Sub Instance ID = 202004
http://logview.odps.com:8080/logview/?h=http://service.sqlflow.com/api&p=my_project&id=1
Sub Instance ID = 202002
http://logview.odps.com:8080/logview/?h=http://service.sqlflow.com/api&p=my_project&id=2
train: running
train: running
train: running
train: running
train: running
train: running
train: 2020-04-17 18:03:57 tensorflow_job:0/0/1[0%]
train: 2020-04-17 18:04:03 tensorflow_job:1/0/1[0%]
`

func TestPickPAILogViwerURL(t *testing.T) {
	a := assert.New(t)
	a.Equal([]string{"http://logview.odps.com:8080/logview/?h=http://service.sqlflow.com/api&p=my_project&id=1",
		"http://logview.odps.com:8080/logview/?h=http://service.sqlflow.com/api&p=my_project&id=2"}, pickPAILogViewerURL(mockPAIOutput))
}

func TestDiagnose(t *testing.T) {
	a := assert.New(t)
	err := diagnose("PAI", mockPAIOutput)
	a.Error(err)
	a.True(strings.HasPrefix(err.Error(), "PAI task failed, please go to check details error logs in the LogViewer website: "))

	rolArnTips := "The role_arn you provide not exists in OSS auth service"
	outputWithOSSError := mockPAIOutput + "\n" + rolArnTips
	err = diagnose("PAI", outputWithOSSError)
	a.Error(err)
	a.True(strings.HasPrefix(err.Error(), "PAI task failed, due to lack of the auth for PAI to access OSS(need to contact your administrator), please go to"))

	tips := "let m3 help u"
	os.Setenv("ERROR_PAI2OSS", tips)
	defer os.Unsetenv("ERROR_PAI2OSS")
	err = diagnose("PAI", outputWithOSSError)
	a.Error(err)
	a.True(strings.HasPrefix(err.Error(), "PAI task failed, "+tips+", please go to check"))
}
