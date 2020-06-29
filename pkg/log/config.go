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
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Formatter is for user to specific the log formatter
type Formatter int

const (
	// TextFormatter means unordered fields
	TextFormatter Formatter = iota
	// OrderedTextFormatter writes the fields(only but not level&msg) orderly
	OrderedTextFormatter
	// JSON, easy to support but we don't need it right now
)

// InitLogger set the output and formatter
func InitLogger(filename string, f Formatter) {
	setOutput(filename)
	if f == OrderedTextFormatter {
		fm := &orderedFieldsTextFormatter{}
		logrus.SetFormatter(fm)
	}
}

// setOutput sets log output to filename globally.
// filename="/var/log/sqlflow.log": write the log to file
// filename="": write the file to stdout or stderr
// filename="/dev/null": ignore log message
func setOutput(filename string) {
	filename = strings.Trim(filename, " ")
	if filename == "/dev/null" {
		logrus.SetOutput(ioutil.Discard)
	} else if filename == "" {
		logrus.SetOutput(os.Stdout)
	} else if len(filename) > 0 {
		logrus.SetOutput(&lumberjack.Logger{
			Filename:   filename,
			MaxSize:    32, // megabytes
			MaxBackups: 64,
			MaxAge:     15, // days
			Compress:   true,
		})
	}
}

// orderedFieldsTextFormatter writes the fields(only but not level or msg) orderly
type orderedFieldsTextFormatter struct {
}

func (f *orderedFieldsTextFormatter) Format(logger *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if logger.Buffer != nil {
		b = logger.Buffer
	} else {
		b = &bytes.Buffer{}
	}
	fmt.Fprintf(b, "%s %s msg=\"%s\"", logger.Time.Format("2006-01-02 15:04:05"), logger.Level.String(), logger.Message)

	keys := make([]string, 0, len(logger.Data))
	for k := range logger.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := logger.Data[k]
		_, ok := v.(string)
		if ok {
			fmt.Fprintf(b, " %s=\"%s\"", k, v)
		} else {
			fmt.Fprintf(b, " %s=%v", k, v)
		}
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}
