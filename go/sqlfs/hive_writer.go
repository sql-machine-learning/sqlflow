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

package sqlfs

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"

	"sqlflow.org/sqlflow/go/database"
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

func hdfsCmd(hdfsCmd, user, passwd, namenodeAddr string) *exec.Cmd {
	hdfsEnv := os.Environ()
	if user != "" {
		hdfsEnv = append(hdfsEnv,
			fmt.Sprintf("HADOOP_USER_NAME=%s", user),
			fmt.Sprintf("HADOOP_USER_PASSWORD=%s", passwd))
	}
	// hdfs command prefix: hdfs dfs ....
	cmdStr := "hdfs dfs"
	// use the specified namenode addr by `-fs hdfs://<ip>:<port>`
	// or use the hdfs-cli configuration file without `-fs` argument
	if namenodeAddr != "" {
		cmdStr = fmt.Sprintf("%s -fs hdfs://%s", cmdStr, namenodeAddr)
	}
	cmdStr = fmt.Sprintf("%s %s", cmdStr, hdfsCmd)

	command := strings.Split(cmdStr, " ")
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = hdfsEnv
	return cmd
}

func uploadCSVFile(csv *os.File, db *sql.DB, hivePath, table, user, passwd, namenodeAddr string) func() error {
	return func() error {
		defer func() {
			csv.Close()
			os.Remove(csv.Name())
		}()

		hdfsPath := path.Join(hivePath, table)

		cmd := hdfsCmd(fmt.Sprintf("-mkdir -p %s", hdfsPath), user, passwd, namenodeAddr)
		if _, e := cmd.CombinedOutput(); e != nil {
			return fmt.Errorf("failed %s: %v", cmd, e)
		}

		defer func() {
			cmd := hdfsCmd(fmt.Sprintf("-rm -r -f %s", hdfsPath), user, passwd, namenodeAddr)
			cmd.CombinedOutput()
		}()

		cmd = hdfsCmd(fmt.Sprintf("-copyFromLocal %s %s", csv.Name(), hdfsPath), user, passwd, namenodeAddr)
		if _, e := cmd.CombinedOutput(); e != nil {
			return fmt.Errorf("failed %s: %v", cmd, e)
		}

		_, e := db.Exec(fmt.Sprintf("LOAD DATA INPATH '%s' OVERWRITE INTO TABLE %s", hdfsPath, table))
		return e
	}
}

func getHdfsParams(connStr string) (namenodeAddr,
	hiveLocation, user, passwd string, e error) {
	uri, e := url.Parse(connStr)
	if e != nil {
		return
	}
	if uri.User != nil {
		user = uri.User.Username()
		passwd, _ = uri.User.Password()
	}
	query := uri.Query()
	namenodeAddr = query.Get("hdfs_namenode_addr")
	hiveLocation = query.Get("hive_location")
	return
}

func newHiveWriter(db *database.DB, table string, bufSize int) (io.WriteCloser, error) {
	namenodeAddr, hivePath, user, passwd, e := getHdfsParams(db.DataSourceName)
	if e != nil {
		return nil, e
	}
	if e := dropTableIfExists(db.DB, table); e != nil {
		return nil, fmt.Errorf("cannot drop table %s: %v", table, e)
	}
	if e := createTable(db, table); e != nil {
		return nil, fmt.Errorf("cannot create table %s: %v", table, e)
	}

	flush, csv, e := flushToCSV()
	if e != nil {
		return nil, e
	}
	upload := uploadCSVFile(csv, db.DB, hivePath, table, user, passwd, namenodeAddr)
	return newFlushWriteCloser(flush, upload, bufSize), nil
}
