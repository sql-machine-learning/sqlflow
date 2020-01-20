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

package sql

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"sqlflow.org/goalisa"
	"sqlflow.org/gomaxcompute"
	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/sql/codegen/pai"
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
			_, err = db.Exec(fmt.Sprintf("DROP TABLE %s", tbName))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func getDatabaseNameFromDSN(dataSource string) (string, error) {
	driverName, datasourceName, err := database.ParseURL(dataSource)
	if err != nil {
		return "", err
	}
	if driverName == "maxcompute" {
		cfg, err := gomaxcompute.ParseDSN(datasourceName)
		if err != nil {
			return "", err
		}
		return cfg.Project, nil
	} else if driverName == "alisa" {
		cfg, err := goalisa.ParseDSN(datasourceName)
		if err != nil {
			return "", err
		}
		return cfg.Project, nil
	}
	return "", fmt.Errorf("driver should be in ['maxcompute', 'alisa']")
}

func createTempTrainAndValTable(trainSelect, validSelect, datasource string) (string, string, error) {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(trainSelect, datasource)
	if err != nil {
		return "", "", err
	}
	tmpTrainTable := strings.Join([]string{dbName, tableName}, ".")
	tmpValTable := ""
	if validSelect != "" {
		dbName, tableName, err := createTmpTableFromSelect(validSelect, datasource)
		if err != nil {
			return "", "", err
		}
		tmpValTable = strings.Join([]string{dbName, tableName}, ".")
	}
	return tmpTrainTable, tmpValTable, nil
}

// Possible situations:
//
// 1. argo mode server: generate a step running: repl -e "repl -e \"select * from xx to train\""
// 2. non-argo mode server | repl -e: create tmp table in go, and use it to train
func (s *paiSubmitter) ExecuteTrain(cl *ir.TrainStmt) (e error) {
	cl.TmpTrainTable, cl.TmpValidateTable, e = createTempTrainAndValTable(cl.Select, cl.ValidationSelect, s.Session.DbConnStr)
	if e != nil {
		return
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

	// format resultTable name to "db.table" to let the codegen form a submitting
	// argument of format "odps://project/tables/table_name"
	resultTableParts := strings.Split(cl.ResultTable, ".")
	if len(resultTableParts) == 1 {
		dbName, err := getDatabaseNameFromDSN(s.Session.DbConnStr)
		if err != nil {
			return err
		}
		cl.ResultTable = fmt.Sprintf("%s.%s", dbName, cl.ResultTable)
	}
	if e := createPredictionTableFromIR(cl, s.Db, s.Session); e != nil {
		return e
	}
	isDeepModel, err := ossModelFileExists(cl.Using)
	if err != nil {
		return err
	}
	code, e := pai.Predict(cl, s.Session, cl.Using, s.Cwd, isDeepModel)
	if e != nil {
		return e
	}
	return s.runCommand(code)
}

func (s *paiSubmitter) ExecuteExplain(cl *ir.ExplainStmt) error {
	// TODO(typhoonzero): Do **NOT** create tmp table when the select statement is like:
	// "SELECT fields,... FROM table"
	dbName, tableName, err := createTmpTableFromSelect(cl.Select, s.Session.DbConnStr)
	if err != nil {
		return err
	}
	cl.TmpExplainTable = strings.Join([]string{dbName, tableName}, ".")
	defer dropTmpTables([]string{cl.TmpExplainTable}, s.Session.DbConnStr)

	isDeepModel, err := ossModelFileExists(cl.ModelName)
	if err != nil {
		return err
	}
	// format resultTable name to "db.table" to let the codegen form a submitting
	// argument of format "odps://project/tables/table_name"
	if cl.Into != "" {
		resultTableParts := strings.Split(cl.Into, ".")
		if len(resultTableParts) == 1 {
			dbName, err := getDatabaseNameFromDSN(s.Session.DbConnStr)
			if err != nil {
				return err
			}
			cl.Into = fmt.Sprintf("%s.%s", dbName, cl.Into)
		}
		db, err := database.OpenAndConnectDB(s.Session.DbConnStr)
		if err != nil {
			return err
		}
		err = createExplainResultTable(db, cl, cl.Into, isDeepModel)
		if err != nil {
			return err
		}
	}

	code, e := pai.Explain(cl, s.Session, cl.ModelName, s.Cwd, isDeepModel)
	if e != nil {
		return e
	}
	return s.runCommand(code)
}

func ossModelFileExists(modelName string) (bool, error) {
	// FIXME(typhoonzero): if the model not exist on OSS, assume it's a random forest model
	// should use a general method to fetch the model and see the model type.
	endpoint := os.Getenv("SQLFLOW_OSS_ENDPOINT")
	ak := os.Getenv("SQLFLOW_OSS_AK")
	sk := os.Getenv("SQLFLOW_OSS_SK")
	if endpoint == "" || ak == "" || sk == "" {
		return false, fmt.Errorf("must define SQLFLOW_OSS_ENDPOINT, SQLFLOW_OSS_AK, SQLFLOW_OSS_SK when using submitter pai")
	}
	// NOTE(typhoonzero): PAI Tensorflow need SQLFLOW_OSS_CHECKPOINT_DIR, get bucket name from it
	ossCheckpointDir := os.Getenv("SQLFLOW_OSS_CHECKPOINT_DIR")
	ckptParts := strings.Split(ossCheckpointDir, "?")
	if len(ckptParts) != 2 {
		return false, fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR got wrong format")
	}
	urlParts := strings.Split(ckptParts[0], "://")
	if len(urlParts) != 2 {
		return false, fmt.Errorf("SQLFLOW_OSS_CHECKPOINT_DIR got wrong format")
	}
	bucketName := strings.Split(urlParts[1], "/")[0]

	cli, err := oss.New(endpoint, ak, sk)
	if err != nil {
		return false, err
	}
	bucket, err := cli.Bucket(bucketName)
	if err != nil {
		return false, err
	}
	ret, err := bucket.IsObjectExist(modelName + "/sqlflow_model_desc")
	return ret, err
}

func createExplainResultTable(db *database.DB, ir *ir.ExplainStmt, tableName string, isDeepModel bool) error {
	dropStmt := fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, tableName)
	if _, e := db.Exec(dropStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", dropStmt, e)
	}
	createStmt := ""
	if !isDeepModel {
		columnDef := ""
		if db.DriverName == "mysql" {
			columnDef = "(feature VARCHAR(255), dfc FLOAT, gain FLOAT)"
		} else {
			// Hive & MaxCompute
			columnDef = "(feature STRING, dfc STRING, gain STRING)"
		}
		createStmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s %s;`, tableName, columnDef)
	} else {
		// create table to record shap values for every feature for each sample.
		flds, _, err := getColumnTypes(ir.Select, db)
		if err != nil {
			return err
		}
		columnDefList := []string{}
		labelCol, ok := ir.Attributes["label_col"]
		if !ok {
			return fmt.Errorf("need to specify WITH label_col=lable_col_name when explaining deep models")
		}
		for _, fieldName := range flds {
			if fieldName != labelCol {
				columnDefList = append(columnDefList, fmt.Sprintf("%s STRING", fieldName))
			}
		}
		createStmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s);`, tableName, strings.Join(columnDefList, ","))
	}
	if _, e := db.Exec(createStmt); e != nil {
		return fmt.Errorf("failed executing %s: %q", createStmt, e)
	}
	return nil
}

func (s *paiSubmitter) GetTrainStmtFromModel() bool { return false }
