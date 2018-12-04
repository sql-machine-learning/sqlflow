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
	trainSelect = testStandardSelectStmt + `TRAIN DNNClassifier
WITH
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN
  employee.name,
  bucketize(last_name, 1000),
  cross(embedding(emplyoee.name), bucketize(last_name, 1000))
LABEL employee.salary
INTO
  my_dnn_model
;
`
	predictSelect = testStandardSelectStmt + `PREDICT db.table.field 
USING my_dnn_model;`
)

func TestStandardSelect(t *testing.T) {
	a := assert.New(t)
	var pr extendedSelect
	a.NotPanics(func() {
		pr = Parse(testStandardSelectStmt + ";")
	})
	a.False(pr.extended)
	a.Equal([]string{"employee.age", "last_name", "salary"},
		pr.fields)
	a.Equal([]string{"employee"}, pr.tables)
	a.Equal("100", pr.limit)
	a.Equal(AND, pr.where.sexp[0].typ)
	a.Equal('<', rune(pr.where.sexp[1].sexp[0].typ))
	a.Equal('=', rune(pr.where.sexp[2].sexp[0].typ))
	a.Equal(`employee.age % 10 < (salary / 10000) AND `+
		`strings.Upper(last_name) = "WANG"`,
		pr.where.String())
}

func TestTrainParser(t *testing.T) {
	a := assert.New(t)
	var pr extendedSelect
	a.NotPanics(func() {
		pr = Parse(trainSelect)
	})
	a.True(pr.extended)
	a.True(pr.train)
	a.Equal("DNNClassifier", pr.estimator)
	a.Equal("[10, 20]", pr.attrs["hidden_units"].String())
	a.Equal("3", pr.attrs["n_classes"].String())
	a.Equal(`employee.name`,
		pr.columns[0].String())
	a.Equal(`bucketize(last_name, 1000)`,
		pr.columns[1].String())
	a.Equal(
		`cross(embedding(emplyoee.name), bucketize(last_name, 1000))`,
		pr.columns[2].String())
	a.Equal("employee.salary", pr.label)
	a.Equal("my_dnn_model", pr.save)
}

func TestPredictParser(t *testing.T) {
	a := assert.New(t)
	var pr extendedSelect
	a.NotPanics(func() {
		pr = Parse(predictSelect)
	})
	a.True(pr.extended)
	a.False(pr.train)
	a.Equal("my_dnn_model", pr.model)
	a.Equal("db.table.field", pr.into)
}

func TestSelectStarAndPrint(t *testing.T) {
	a := assert.New(t)
	var pr extendedSelect
	a.NotPanics(func() {
		pr = Parse(`SELECT *, b FROM a LIMIT 10;`)
	})
	a.Equal(2, len(pr.fields))
	a.Equal("*", pr.fields[0])
	a.False(pr.extended)
	a.False(pr.train)
	a.Equal("SELECT *, b\nFROM a\nLIMIT 10;", pr.standardSelect.String())
}
