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
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testStandardSelectStmt = `
SELECT employee.age, last_name, salary
FROM   employee
LIMIT  100
WHERE
  employee.age % 10 < (salary / 10000)
  AND
  strings.Upper(last_name) = "WANG"
`
	testTrainSelect = testStandardSelectStmt + `TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN
  employee.name,
  bucketize(last_name, 1000),
  cross(embedding(emplyoee.name), bucketize(last_name, 1000))
LABEL "employee.salary"
INTO sqlflow_models.my_dnn_model;
`
	testMultiColumnTrainSelect = testStandardSelectStmt + `TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN
  employee.name,
  bucketize(last_name, 1000),
  cross(embedding(emplyoee.name), bucketize(last_name, 1000))
COLUMN
  cross(embedding(emplyoee.name), bucketize(last_name, 1000)) FOR C2
LABEL employee.salary
INTO sqlflow_models.my_dnn_model;
`
	testPredictSelect = testStandardSelectStmt + `PREDICT db.table.field
USING sqlflow_models.my_dnn_model;`
)

func TestStandardSelect(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testStandardSelectStmt + ";")
	a.NoError(e)
	a.False(r.extended)
	a.Equal([]string{"employee.age", "last_name", "salary"},
		r.fields)
	a.Equal([]string{"employee"}, r.tables)
	a.Equal("100", r.limit)
	a.Equal(AND, r.where.sexp[0].typ)
	a.Equal('<', rune(r.where.sexp[1].sexp[0].typ))
	a.Equal('=', rune(r.where.sexp[2].sexp[0].typ))
	a.Equal(`employee.age % 10 < (salary / 10000) AND `+
		`strings.Upper(last_name) = "WANG"`,
		r.where.String())
}

func TestTrainParser(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testTrainSelect)
	a.NoError(e)
	a.True(r.extended)
	a.True(r.train)
	a.Equal("DNNClassifier", r.estimator)
	a.Equal("[10, 20]", r.attrs["hidden_units"].String())
	a.Equal("3", r.attrs["n_classes"].String())
	a.Equal(`employee.name`,
		r.columns["feature_columns"][0].String())
	a.Equal(`bucketize(last_name, 1000)`,
		r.columns["feature_columns"][1].String())
	a.Equal(
		`cross(embedding(emplyoee.name), bucketize(last_name, 1000))`,
		r.columns["feature_columns"][2].String())
	a.Equal("employee.salary", r.label)
	a.Equal("sqlflow_models.my_dnn_model", r.save)
}

func TestMultiColumnTrainParser(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testMultiColumnTrainSelect)
	a.NoError(e)
	a.True(r.extended)
	a.True(r.train)
	a.Equal("DNNClassifier", r.estimator)
	a.Equal("[10, 20]", r.attrs["hidden_units"].String())
	a.Equal("3", r.attrs["n_classes"].String())
	a.Equal(`employee.name`,
		r.columns["feature_columns"][0].String())
	a.Equal(`bucketize(last_name, 1000)`,
		r.columns["feature_columns"][1].String())
	a.Equal(
		`cross(embedding(emplyoee.name), bucketize(last_name, 1000))`,
		r.columns["feature_columns"][2].String())
	a.Equal(
		`cross(embedding(emplyoee.name), bucketize(last_name, 1000))`,
		r.columns["C2"][0].String())
	a.Equal("employee.salary", r.label)
	a.Equal("sqlflow_models.my_dnn_model", r.save)
}

func TestPredictParser(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(testPredictSelect)
	a.NoError(e)
	a.True(r.extended)
	a.False(r.train)
	a.Equal("sqlflow_models.my_dnn_model", r.model)
	a.Equal("db.table.field", r.into)
}

func TestSelectStarAndPrint(t *testing.T) {
	a := assert.New(t)
	r, e := newParser().Parse(`SELECT *, b FROM a LIMIT 10;`)
	a.NoError(e)
	a.Equal(2, len(r.fields))
	a.Equal("*", r.fields[0])
	a.False(r.extended)
	a.False(r.train)
	a.Equal("SELECT *, b\nFROM a\nLIMIT 10;", r.standardSelect.String())
}

func TestStandardDropTable(t *testing.T) {
	a := assert.New(t)
	_, e := newParser().Parse(`DROP TABLE PREDICT`)
	a.Error(e)
	// Note: currently, our parser doesn't accept anything statements other than SELECT.
	// It will support parsing any SQL statements and even dialects in the future.
}
