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
	"log"
	"math/rand"
	"strings"

	"sqlflow.org/gomaxcompute"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/parser"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

type paiSubmitter struct{ *defaultSubmitter }

func randStringRunes(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const lettersAndDigits = letters + "0123456789"
	b := make([]byte, n)
	// do not start from digit
	b[0] = letters[rand.Intn(len(letters))]
	for i := 1; i < len(b); i++ {
		b[i] = lettersAndDigits[rand.Intn(len(lettersAndDigits))]
	}
	return string(b)
}

func createTmpTableFromSelect(selectStmt, dataSource string) (string, string, error) {
	db, err := database.OpenAndConnectDB(dataSource)
	defer db.Close()
	if err != nil {
		return "", "", err
	}
	tableName := randStringRunes(16)
	// FIXME(typhoonzero): only work if specify database name in connect string.
	databaseName, err := getDatabaseNameFromDSN(dataSource)
	// NOTE(typhoonzero): MaxCompute do not support "CREATE	TABLE XXX AS (SELECT ...)"
	createSQL := fmt.Sprintf("CREATE TABLE %s AS %s", tableName, selectStmt)
	log.Printf(createSQL)
	_, err = db.Exec(createSQL)
	return databaseName, tableName, err
}

func dropTmpTables(tableNames []string, dataSource string) error {
	db, err := database.OpenAndConnectDB(dataSource)
	defer db.Close()
	if err != nil {
		return err
	}
	for _, tbName := range tableNames {
		if tbName != "" {
			_, err = db.Exec("DROP TABLE %s", tbName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getDatabaseNameFromDSN(dataSource string) (string, error) {
	dsParts := strings.Split(dataSource, "://")
	if len(dsParts) != 2 {
		return "", fmt.Errorf("error datasource format, should be maxcompute://u:p@uri, but got: %s", dataSource)
	}
	conf, err := gomaxcompute.ParseDSN(dsParts[1])
	if err != nil {
		return "", err
	}
	return conf.Project, nil
}

// Possible situations:
//
// 1. argo mode server: generate a step running: repl -e "repl -e \"select * from xx to train\""
// 2. non-argo mode server | repl -e: create tmp table in go, and use it to train

func (s *paiSubmitter) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cl.TmpTrainTable = strings.Join([]string{dbName, tableName}, ".")
	if cl.ValidationSelect != "" {
		dbName, tableName, err := createTmpTableFromSelect(cl.ValidationSelect, s.Session.DbConnStr)
		if err != nil {
			return err
		}
		cl.TmpValidateTable = strings.Join([]string{dbName, tableName}, ".")
	}
	defer dropTmpTables([]string{cl.TmpTrainTable, cl.TmpValidateTable}, s.Session.DbConnStr)

	code, e := pai.Train(cl, s.Session, cl.Into, s.Cwd)
	if e != nil {
		return e
	}
	return s.runCommand(code)
}

func (s *paiSubmitter) ExecutePredict(cl *ir.PredictStmt) error {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cl.TmpPredictTable = strings.Join([]string{dbName, tableName}, ".")
	defer dropTmpTables([]string{cl.TmpPredictTable}, s.Session.DbConnStr)

	// TODO(typhoonzero): remove below twice parse when all submitters moved to IR.
	pr, e := parser.ParseOneStatement("maxcompute", cl.OriginalSQL)
	if e != nil {
		return e
	}
	if e = createPredictionTableFromIR(cl, s.Db, s.Session); e != nil {
		return e
	}
	code, e := pai.Predict(cl, s.Session, pr.Model, s.Cwd)
	if e != nil {
		return e
	}
	return s.runCommand(code)
}

func (s *paiSubmitter) GetTrainStmtFromModel() bool { return false }
func init()                                         { submitterRegistry["pai"] = &paiSubmitter{&defaultSubmitter{}} }
