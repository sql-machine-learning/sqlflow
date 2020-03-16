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
	"github.com/sirupsen/logrus"
)

// Entry is the final or intermediate Logrus logging entry.
// It contains all the fields passed with WithField{,s}.
type Entry struct {
	*logrus.Entry
}

// Fields type, used to pass to `WithFields`.
type Fields = logrus.Fields

// WithFields returns log.Entry
func WithFields(fields Fields) *Entry {
	return &Entry{logrus.WithFields(fields)}
}

// Info logs a message at level Info
func (entry *Entry) Info(args ...interface{}) {
	entry.Log(logrus.InfoLevel, args...)
}

// Infof logs a message at level Info
func (entry *Entry) Infof(format string, args ...interface{}) {
	entry.Logf(logrus.InfoLevel, format, args...)
}

// Error logs a message at level Error
func (entry *Entry) Error(args ...interface{}) {
	entry.Log(logrus.ErrorLevel, args...)
}

// Errorf logs a message at level Error
func (entry *Entry) Errorf(format string, args ...interface{}) {
	entry.Logf(logrus.ErrorLevel, format, args...)
}

// Fatal logs a message at level Fatal then the process will exit with status set to 1.
func (entry *Entry) Fatal(args ...interface{}) {
	entry.Log(logrus.FatalLevel, args...)
}

// Fatalf logs a message at level Fatal then the process will exit with status set to 1.
func (entry *Entry) Fatalf(format string, args ...interface{}) {
	entry.Logf(logrus.FatalLevel, format, args...)
}
