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

package experimental

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"sqlflow.org/sqlflow/go/database"
	pb "sqlflow.org/sqlflow/go/proto"
)

func TestExperimentalXGBCodegen(t *testing.T) {
	a := assert.New(t)
	if os.Getenv("SQLFLOW_TEST_DB") != "mysql" {
		t.Skipf("skip TestExperimentalXGBCodegen of DB type %s", os.Getenv("SQLFLOW_TEST_DB"))
	}
	// test without COLUMN clause
	sql := "SELECT * FROM iris.train TO TRAIN xgboost.gbtree WITH objective=\"multi:softmax\",num_class=3 LABEL class INTO sqlflow_models.xgb_classification;"
	s := &pb.Session{DbConnStr: database.GetTestingMySQLURL()}
	coulerCode, err := GenerateCodeCouler(sql, s)
	if err != nil {
		t.Errorf("error %s", err)
	}
	a.True(strings.Contains(coulerCode, `couler.run_script(image="sqlflow/sqlflow:step", command="bash", source="\n".join(codes), env=step_envs, resources=resources)`))

	// test with COLUMN clause
	sql = "SELECT * FROM iris.train TO TRAIN xgboost.gbtree WITH objective=\"multi:softmax\",num_class=3 COLUMN petal_length LABEL class INTO sqlflow_models.xgb_classification;"
	coulerCode, err = GenerateCodeCouler(sql, s)
	if err != nil {
		t.Errorf("error %s", err)
	}
	expected := `feature_column_map = {"feature_columns":[runtime.feature.column.NumericColumn(runtime.feature.field_desc.FieldDesc(name="petal_length", dtype=runtime.feature.field_desc.DataType.FLOAT32, dtype_weight=runtime.feature.field_desc.DataType.INT64, delimiter="", delimiter_kv="", format="", shape=[1], is_sparse=False, vocabulary=[]))]}`
	a.True(strings.Contains(coulerCode, expected))
}

func TestGeneratePyDbConnStr(t *testing.T) {
	mysqlSession := &pb.Session{
		DbConnStr: database.GetTestingMySQLURL(),
	}

	dbConnStr, err := GeneratePyDbConnStr(mysqlSession)
	assert.NoError(t, err)
	assert.Equal(t, database.GetTestingMySQLURL(), dbConnStr)

	hiveSession := &pb.Session{
		DbConnStr:        "hive://root:root@127.0.0.1:10000/iris?auth=NOSASL",
		HiveLocation:     "/sqlflowtmp",
		HdfsNamenodeAddr: "192.168.1.1:8020",
		HdfsUser:         "sqlflow_admin",
		HdfsPass:         "sqlflow_pass",
	}

	dbConnStr, err = GeneratePyDbConnStr(hiveSession)
	assert.NoError(t, err)
	assert.Equal(t, `hive://root:root@127.0.0.1:10000/iris?auth=NOSASL&hdfs_namenode_addr=192.168.1.1%3A8020&hdfs_pass=sqlflow_pass&hdfs_user=sqlflow_admin&hive_location=%2Fsqlflowtmp`, dbConnStr)
}

func TestGetPyFuncBody(t *testing.T) {
	program := `
def test1():
    return "test1"

def test2():
    a = "test2"
    return "test2"
`

	code1, err := GetPyFuncBody(program, "test1")
	assert.NoError(t, err)
	assert.Equal(t, `return "test1"`, code1)

	code2, err := GetPyFuncBody(program, "test2")
	assert.NoError(t, err)
	assert.Equal(t, `a = "test2"
return "test2"`, code2)

	_, err = GetPyFuncBody(program, "test3")
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), program))
}
