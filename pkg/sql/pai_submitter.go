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
	"regexp"
	"strings"

	"sqlflow.org/gomaxcompute"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/parser"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
	"sqlflow.org/sqlflow/pkg/sql/ir"
)

type paiSubmitter struct{ *defaultSubmitter }

func createTmpTableFromSelect(selectStmt, dataSource string) (string, string, error) {
	db, err := database.OpenAndConnectDB(dataSource)
	defer db.Close()
	if err != nil {
		return "", "", err
	}
	tableName = "xxxx_wuyi"
	// FIXME(typhoonzero): only work if specify database name in connect string.
	databaseName, err = getDatabaseNameFromDSN(dataSource)

	createSQL := fmt.Sprintf("CREATE TABLE %s AS %s", tableName, selectStmt)
	log.Printf(createSQL)
	_, err = db.Exec(createSQL)
	return databaseName, tableName, err
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

// getTableFromSelect get the database name and table name from simple select statement:
// SELECT * FROM db.table
func getTableFromSelect(dataSource, trainSelect string) (string, string, error) {
	fromRegex, err := regexp.Compile("SELECT\\s\\*\\sFROM[\\s\\n]+([\\w\\.]*)")
	if err != nil {
		return "", "", err
	}
	matches := fromRegex.FindAllStringSubmatch(trainSelect, -1)
	if len(matches) != 1 {
		return "", "", fmt.Errorf("only support simple SQL query, but got %s", trainSelect)
	}
	tableFull := matches[0][1]
	databaseName := ""
	tableName := ""
	tableParts := strings.Split(tableFull, ".")
	if len(tableParts) == 2 {
		databaseName = tableParts[0]
		tableName = tableParts[1]
	} else {
		databaseName, err = getDatabaseNameFromDSN(dataSource)
		if err != nil {
			return "", "", err
		}
		tableName = tableFull
	}
	return databaseName, tableName, nil
}

func (s *paiSubmitter) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	if cl.NeedCreateTmpTable() {
		if s.isArgoMode {
			// In argo mode the temp table will create the table in the training step,
			// and the train SQL is always like "SELECT * FROM db.table TO TRAIN ..." (see couler/template.go)
			// so parse the table name from "SELECT * FROM db.table"
			db, tb, err := getTableFromSelect(cl.DataSource, cl.Select)
			cl.TmpTrainTable = strings.Join([]string{db, tb}, ".")
			if cl.ValidationSelect != "" {
				db, tb, err := getTableFromSelect(cl.DataSource, cl.ValidationSelect)
				cl.TmpValidateTable = strings.Join([]string{db, tb}, ".")
			}
		} else {
			// Create a temp table here if not using argo mode.
			dbName, tableName, err := createTmpTableFromSelect(cl.Select, cl.DataSource)
			if err != nil {
				return err
			}
			cl.TmpTrainTable = strings.Join([]string{dbName, tableName}, ".")
			if cl.ValidationSelect != "" {
				dbName, tableName, err := createTmpTableFromSelect(cl.ValidationSelect, cl.DataSource)
				if err != nil {
					return err
				}
				cl.TmpValidateTable = strings.Join([]string{dbName, tableName}, ".")
			}
		}
	}

	code, e := pai.Train(cl, cl.Into, s.Cwd)
	if e != nil {
		return e
	}
	return s.runCommand(code)
}

func (s *paiSubmitter) ExecutePredict(cl *ir.PredictStmt) error {
	// TODO(typhoonzero): remove below twice parse when all submitters moved to IR.
	pr, e := parser.ParseOneStatement("maxcompute", cl.OriginalSQL)
	if e != nil {
		return e
	}
	if e = createPredictionTableFromIR(cl, s.Db, s.Session); e != nil {
		return e
	}
	code, e := pai.Predict(cl, pr.Model, s.Cwd)
	if e != nil {
		return e
	}
	return s.runCommand(code)
}

func (s *paiSubmitter) GetTrainStmtFromModel() bool { return false }
func init()                                         { submitterRegistry["pai"] = &paiSubmitter{&defaultSubmitter{}} }
