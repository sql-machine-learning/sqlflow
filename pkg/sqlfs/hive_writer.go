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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	pb "sqlflow.org/sqlflow/pkg/server/proto"
)

// HiveWriter implements io.WriteCloser.
type HiveWriter struct {
	Writer
	csvFile *os.File
	session *pb.Session
}

// Write write bytes to sqlfs and returns (num_bytes, error)
func (w *HiveWriter) Write(p []byte) (n int, e error) {
	w.csvFile, e = ioutil.TempFile("/tmp", "sqlflow-sqlfs")
	if e != nil {
		return n, fmt.Errorf("create temporary csv file failed: %v", e)
	}
	for len(p) > 0 {
		fill := bufSize - len(w.buf)
		if fill > len(p) {
			fill = len(p)
		}
		w.buf = append(w.buf, p[:fill]...)
		p = p[fill:]
		n += fill
		if len(w.buf) >= bufSize {
			block := base64.StdEncoding.EncodeToString(w.buf)
			w.csvFile.Write([]byte(fmt.Sprintf("%d\001%s\n", w.flushID, block)))
			w.buf = w.buf[:0]
			w.flushID++
		}
	}

	return n, nil
}

func (w *HiveWriter) hdfsPath() string {
	return fmt.Sprintf("%s/models/%s/", w.session.HiveLocation, w.table)
}

// Close the connection of the sqlfs
func (w *HiveWriter) Close() error {
	defer func() {
		w.csvFile.Close()
		w.db = nil
	}()

	// 1. create a directory on HDFS
	cmd := exec.Command("hdfs", "dfs", "-mkdir", "-p", w.hdfsPath())
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HADOOP_USER_NAME=%s", w.session.HdfsUser),
		fmt.Sprintf("HADOOP_USER_PASSWORD=%s", w.session.HdfsPass))
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	// 2. upload the local csv file to the HDFS path
	cmd = exec.Command("hdfs", "dfs", "-copyFromLocal", w.csvFile.Name(), w.hdfsPath())
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HADOOP_USER_NAME=%s", w.session.HdfsUser),
		fmt.Sprintf("HADOOP_USER_PASSWORD=%s", w.session.HdfsPass))
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	// 3. execute a LOAD statment to load csv to Hive table
	query := fmt.Sprintf("LOAD DATA INNPATH %s OVERWRITE INTO TABLE %s", w.hdfsPath(), w.table)
	if _, e := w.db.Exec(query); e != nil {
		return fmt.Errorf("load data to %s, error:%v", w.table, e)
	}
	return nil
}
