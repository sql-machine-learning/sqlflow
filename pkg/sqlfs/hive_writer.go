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

	pb "sqlflow.org/sqlflow/pkg/proto"
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

// HiveWriter implements io.WriteCloser.
type HiveWriter struct {
	Writer
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
		Writer: Writer{
			db:      db,
			table:   table,
			buf:     make([]byte, 0, bufSize),
			flushID: 0,
		},
		csvFile: csvFile,
		session: session}, nil
}

// Write write bytes to sqlfs and returns (num_bytes, error)
func (w *HiveWriter) Write(p []byte) (n int, e error) {
	n = 0
	for len(p) > 0 {
		fill := bufSize - len(w.buf)
		if fill > len(p) {
			fill = len(p)
		}
		w.buf = append(w.buf, p[:fill]...)
		p = p[fill:]
		n += fill
		if len(w.buf) >= bufSize {
			if e := w.flush(); e != nil {
				return 0, e
			}
		}
	}
	return n, nil
}

func removeHDFSDir(hdfsPath string) error {
	cmd := exec.Command("hdfs", "hdfs", "-rmr", "-p", hdfsPath)
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	return nil
}

func hdfsEnv(username, password string) []string {
	hdfsEnv := os.Environ()
	if username != "" {
		hdfsEnv = append(hdfsEnv,
			fmt.Sprintf("HADOOP_USER_NAME=%s", username),
			fmt.Sprintf("HADOOP_USER_PASSWORD=%s", password))
	}
	return hdfsEnv
}

func createHDFSDir(hdfsPath, username, password string) error {
	cmd := exec.Command("hdfs", "dfs", "-mkdir", "-p", hdfsPath)
	cmd.Env = hdfsEnv(username, password)
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	return nil
}

func uploadFileToHDFS(localFilePath, hdfsPath, username, password string) error {
	cmd := exec.Command("hdfs", "dfs", "-copyFromLocal", localFilePath, hdfsPath)
	cmd.Env = hdfsEnv(username, password)
	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("upload local file into hdfs error: %v", err)
	}
	return nil
}

func loadHDFSfileIntoTable(db *sql.DB, hdfsPath, table string) error {
	query := fmt.Sprintf("LOAD DATA INPATH '%s' OVERWRITE INTO TABLE %s", hdfsPath, table)
	if _, e := db.Exec(query); e != nil {
		return fmt.Errorf("execute query: %s, error: %v", query, e)
	}
	return nil
}

// Close the connection of the sqlfs
func (w *HiveWriter) Close() error {
	if w.db == nil {
		return nil
	}
	defer func() {
		w.csvFile.Close()
		os.Remove(w.csvFile.Name())
		w.db = nil
	}()

	if e := w.flush(); e != nil {
		return e
	}

	// 1. create a directory on HDFS
	if err := createHDFSDir(w.hdfsPath(), w.session.HdfsUser, w.session.HdfsPass); err != nil {
		return fmt.Errorf("create HDFDS dir: %s failed: %v", w.hdfsPath(), err)
	}

	// 2. upload the local csv file to the HDFS directory
	if err := uploadFileToHDFS(w.csvFile.Name(), w.hdfsPath(), w.session.HdfsUser, w.session.HdfsPass); err != nil {
		return fmt.Errorf("upload local file to hdfs failed: %v", err)
	}

	// 3. load hdfs files into hive table
	if err := loadHDFSfileIntoTable(w.db, w.hdfsPath(), w.table); err != nil {
		return fmt.Errorf("load hdfs filie into table failed: %v", err)
	}

	// 4. remove the uploaded csv path on HDFS
	if err := removeHDFSDir(w.hdfsPath()); err != nil {
		return err
	}
	return nil
}

func (w *HiveWriter) hdfsPath() string {
	return fmt.Sprintf("%s/sqlfs/%s/", w.session.HiveLocation, w.table)
}

func (w *HiveWriter) flush() error {
	if len(w.buf) > 0 {
		block := base64.StdEncoding.EncodeToString(w.buf)
		if _, e := w.csvFile.Write([]byte(fmt.Sprintf("%d\001%s\n", w.flushID, block))); e != nil {
			return fmt.Errorf("flush error, %v", e)
		}
		w.buf = w.buf[:0]
		w.flushID++
	}
	return nil
}
