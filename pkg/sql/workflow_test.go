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
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
)

func TestFetchWorkflowLog(t *testing.T) {
	if os.Getenv("SQLFLOW_ARGO_MODE") != "True" {
		t.Skip("argo: skip Argo tests")
	}
	a := assert.New(t)
	modelDir := ""
	a.NotPanics(func() {
		rd := SubmitWorkflow(`select 1; select 1;`, testDB, modelDir, getDefaultSession())
		for r := range rd.ReadAll() {
			switch r.(type) {
			case WorkflowJob:
				job := r.(WorkflowJob)
				a.True(strings.HasPrefix(job.JobID, "sqlflow-couler"))
				// TODO(tony): wait to check if job succeeded.
				// The workflow is currently failed since we haven't configure the data source.

				a.NoError(fetchWorkflowLog(job))

			default:
				a.Fail("SubmitWorkflow should return JobID")
			}
		}
	})
}
