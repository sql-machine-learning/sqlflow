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

package log

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestOrderedTextFormatter(t *testing.T) {
	a := assert.New(t)
	InitLogger("", OrderedTextFormatter)
	b := &bytes.Buffer{}
	logrus.SetOutput(b)

	logger := WithFields(Fields{"a": 1, "z": 26, "c": "7 3", "s": 5, "e": 3, "f": "true"})
	logger.Info("this is a message")
	logContent := b.String()
	expectedWithoutTime := " info msg=\"this is a message\" a=1 c=\"7 3\" e=3 f=\"true\" s=5 z=26\n"
	a.Truef(strings.HasSuffix(logContent, expectedWithoutTime), "must contain: %s, but got: %s", expectedWithoutTime, logContent)
}
