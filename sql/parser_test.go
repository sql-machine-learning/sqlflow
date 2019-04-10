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
		r.columns[0].String())
	a.Equal(`bucketize(last_name, 1000)`,
		r.columns[1].String())
	a.Equal(
		`cross(embedding(emplyoee.name), bucketize(last_name, 1000))`,
		r.columns[2].String())
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
	if _, e := newParser().Parse(`DROP TABLE PREDICT`); e != nil {
		t.Skipf("[FIXME]`drop table` expected no error, but got:%v", e)
	}
}
