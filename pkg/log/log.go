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
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Factory builds Logger
type Factory struct{}

// Logger wraps logrus.Entry
type Logger struct {
	*logrus.Entry
}

// New sets logrus'output with lumberjack
// filename: /path/to/log, e.g. "/var/log/sqlflow.log"
func New(filename string) *Factory {
	if len(strings.Trim(filename, " ")) > 0 {
		logrus.SetOutput(&lumberjack.Logger{
			Filename:   filename,
			MaxSize:    32, // megabytes
			MaxBackups: 64,
			MaxAge:     10, // days
			Compress:   true,
		})
	}
	return &Factory{}
}

// GetLogger returns log.Entry
// TODO(weiguoz): Need stress testing to detect memory leaking and performance.
func (fac *Factory) GetLogger(fields map[string]interface{}) *Logger {
	return &Logger{logrus.WithFields(fields)}
}
