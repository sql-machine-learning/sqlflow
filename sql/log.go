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
	"fmt"
	"io"
	"os"
	"path"

	"github.com/sirupsen/logrus"
)

var log *logrus.Entry

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func init() {
	// Default log to stdout also make SQLFlow a cloud-native service
	logDir := getEnv("SQLFLOW_log_dir", "")
	logLevel := getEnv("SQLFLOW_log_level", "info")

	ll, e := logrus.ParseLevel(logLevel)
	if e != nil {
		ll = logrus.InfoLevel
	}
	var f io.Writer
	if logDir != "" {
		e = os.MkdirAll(logDir, 0744)
		if e != nil {
			log.Panicf("create log directory failed: %v", e)
		}

		f, e = os.Create(path.Join(logDir, fmt.Sprintf("sqlflow-%d.log", os.Getpid())))
		if e != nil {
			log.Panicf("open log file failed: %v", e)
		}
	} else {
		f = os.Stdout
	}

	lg := logrus.New()
	lg.SetOutput(f)
	lg.SetLevel(ll)
	log = lg.WithFields(logrus.Fields{"package": "sql"})
}
