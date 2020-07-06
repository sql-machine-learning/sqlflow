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
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Logger wraps logrus.Entry. It is the final or intermediate Logrus
// logging entry. It contains all the fields passed with WithField{,s}.
type Logger struct {
	*logrus.Entry
}

// Fields type, used to pass to `WithFields`.
type Fields = logrus.Fields

// UUID always used to identify the request.
func UUID() string {
	u, _ := uuid.NewUUID()
	return u.String()
}

// WithFields returns log.Entry
func WithFields(fields Fields) *Logger {
	return &Logger{logrus.WithFields(fields)}
}

// GetDefaultLogger returns loggers without any fields
func GetDefaultLogger() *Logger {
	return &Logger{logrus.WithFields(map[string]interface{}{})}
}

// Info logs a message at level Info
func (l *Logger) Info(args ...interface{}) {
	l.Log(logrus.InfoLevel, args...)
}

// Infof logs a message at level Info
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Logf(logrus.InfoLevel, format, args...)
}

// Error logs a message at level Error
func (l *Logger) Error(args ...interface{}) {
	l.Log(logrus.ErrorLevel, args...)
}

// Errorf logs a message at level Error
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Logf(logrus.ErrorLevel, format, args...)
}

// Fatal logs a message at level Fatal then the process will exit with status set to 1.
func (l *Logger) Fatal(args ...interface{}) {
	l.Log(logrus.FatalLevel, args...)
}

// Fatalf logs a message at level Fatal then the process will exit with status set to 1.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Logf(logrus.FatalLevel, format, args...)
}
