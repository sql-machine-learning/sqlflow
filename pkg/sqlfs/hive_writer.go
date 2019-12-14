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
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
)

func flushToCSV() (func([]byte) error, *os.File, error) {
	csv, e := ioutil.TempFile("", "sqlflow-sqlfs")
	if e != nil {
		return nil, nil, fmt.Errorf("cannot create CSV file: %v", e)
	}

	row := 0
	return func(buf []byte) error {
		if len(buf) > 0 {
			block := base64.StdEncoding.EncodeToString(buf)
			_, e := csv.Write([]byte(fmt.Sprintf("%d\001%s\n", row, block)))
			if e != nil {
				return fmt.Errorf("cannot flush to CSV, %v", e)
			}
			row++
		}
		return nil
	}, csv, nil
}

func uploadCSVFile(csv *os.File, db *sql.DB, hivePath, table, user, passwd string) func() error {
	return func() error {
		defer func() {
			csv.Close()
			os.Remove(csv.Name())
		}()

		hdfsPath := path.Join(hivePath, table)

		hdfsEnv := os.Environ()
		if user != "" {
			hdfsEnv = append(hdfsEnv,
				fmt.Sprintf("HADOOP_USER_NAME=%s", user),
				fmt.Sprintf("HADOOP_USER_PASSWORD=%s", passwd))
		}

		cmd := exec.Command("hdfs", "dfs", "-mkdir", "-p", hdfsPath)
		cmd.Env = hdfsEnv
		if _, e := cmd.CombinedOutput(); e != nil {
			return fmt.Errorf("failed %s: %v", cmd, e)
		}

		cmd = exec.Command("hdfs", "dfs", "-copyFromLocal", csv.Name(), hdfsPath)
		cmd.Env = hdfsEnv
		if _, e := cmd.CombinedOutput(); e != nil {
			return fmt.Errorf("failed %s: %v", cmd, e)
		}

		_, e := db.Exec(fmt.Sprintf("LOAD DATA INPATH '%s' OVERWRITE INTO TABLE %s", hdfsPath, table))
		return e
	}
}

func newHiveWriter(db *sql.DB, hivePath, table, user, passwd string) (io.WriteCloser, error) {
	if e := dropTable(db, table); e != nil {
		return nil, fmt.Errorf("cannot drop table %s: %v", table, e)
	}
	if e := createTable(db, "hive", table); e != nil {
		return nil, fmt.Errorf("cannot create table %s: %v", table, e)
	}

	flush, csv, e := flushToCSV()
	if e != nil {
		return nil, e
	}
	upload := uploadCSVFile(csv, db, hivePath, table, user, passwd)
	const bufSize = 32 * 1024
	return newFlushWriteCloser(flush, upload, bufSize), nil
}
