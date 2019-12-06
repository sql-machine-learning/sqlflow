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

package sqlfs

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	pb "sqlflow.org/sqlflow/pkg/proto"
)

// HiveWriter implements io.WriteCloser.
type HiveWriter struct {
	db      *sql.DB
	table   string
	csvFile *os.File
	session *pb.Session
}

// NewHiveWriter returns a Hive Writer object
func NewHiveWriter(db *sql.DB, table string, session *pb.Session) (*HiveWriter, error) {
	csvFile, e := ioutil.TempFile("/tmp", "sqlflow-sqlfs")
	if e != nil {
		return nil, fmt.Errorf("create temporary csv file failed: %v", e)
	}
	return &HiveWriter{
		db:      db,
		table:   table,
		csvFile: csvFile,
		session: session}, nil
}

func uploadHDFSWrapUp(w *HiveWriter) func() error {
	return func() error {
		if w.db == nil {
			return nil
		}
		defer func() {
			w.csvFile.Close()
			os.Remove(w.csvFile.Name())
			w.db = nil
		}()

		// 1. create a directory on HDFS
		cmd := exec.Command("hdfs", "dfs", "-mkdir", "-p", w.hdfsPath())
		hdfsEnv := os.Environ()
		if w.session.HdfsUser != "" {
			hdfsEnv = append(hdfsEnv,
				fmt.Sprintf("HADOOP_USER_NAME=%s", w.session.HdfsUser),
				fmt.Sprintf("HADOOP_USER_PASSWORD=%s", w.session.HdfsPass))
		}
		cmd.Env = hdfsEnv
		if _, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf(`execute "hdfs dfs -mkdir -p %s" failed: %v `, w.hdfsPath(), err)
		}
		// 2. upload the local csv file to the HDFS path
		cmd = exec.Command("hdfs", "dfs", "-copyFromLocal", w.csvFile.Name(), w.hdfsPath())
		cmd.Env = hdfsEnv
		if _, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("upload local file into hdfs error: %v", err)
		}
		// 3. execute a LOAD statement to load csv to Hive table
		query := fmt.Sprintf("LOAD DATA INPATH '%s' OVERWRITE INTO TABLE %s", w.hdfsPath(), w.table)
		if _, e := w.db.Exec(query); e != nil {
			return fmt.Errorf("execute query: %s, error: %v", query, e)
		}
		return nil
	}
}

func (w *HiveWriter) hdfsPath() string {
	return fmt.Sprintf("%s/models/%s/", w.session.HiveLocation, w.table)
}

func flushToCSV(w *HiveWriter) func([]byte, int) error {
	return func(buf []byte, flushes int) error {
		block := base64.StdEncoding.EncodeToString(buf)
		if _, e := w.csvFile.Write([]byte(fmt.Sprintf("%d\001%s\n", flushes, block))); e != nil {
			return fmt.Errorf("flush error, %v", e)
		}
		return nil
	}
}
