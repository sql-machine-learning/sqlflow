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
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"sync"

	"sqlflow.org/sqlflow/pkg/pipe"
)

var rePyDiagnosis = regexp.MustCompile("sqlflow_submitter.tensorflow.diag.SQLFlowDiagnostic: (.*)")

// Figures contains analyzed figures as strings
type Figures struct {
	Image string
	Text  string
}

type logChanWriter struct {
	wr   *pipe.Writer
	m    sync.Mutex
	buf  bytes.Buffer
	prev string
}

func (cw *logChanWriter) Write(p []byte) (n int, err error) {
	// Both cmd.Stdout and cmd.Stderr are writing to cw
	cw.m.Lock()
	defer cw.m.Unlock()

	n, err = cw.buf.Write(p)
	if err != nil {
		return n, err
	}
	for {
		line, err := cw.buf.ReadString('\n')
		cw.prev = cw.prev + line
		// ReadString returns err != nil if and only if the returned Data
		// does not end in delim.
		if err != nil {
			break
		}
		if err := cw.wr.Write(cw.prev); err != nil {
			return len(cw.prev), err
		}
		cw.prev = ""
	}
	return n, nil
}

func (cw *logChanWriter) Close() {
	if len(cw.prev) > 0 {
		cw.wr.Write(cw.prev)
		cw.prev = ""
	}
}

func readExplainResult(cwd string, wr *pipe.Writer) error {
	content, err := ioutil.ReadFile(path.Join(cwd, "summary.png"))
	if err != nil {
		return err
	}
	img := fmt.Sprintf("<div align='center'><img src='data:image/png;base64,%s' /></div>",
		base64.StdEncoding.EncodeToString(content))
	txt, err := ioutil.ReadFile(path.Join(cwd, "summary.txt"))
	if err != nil {
		return err
	}
	return wr.Write(Figures{img, string(txt)})
}
