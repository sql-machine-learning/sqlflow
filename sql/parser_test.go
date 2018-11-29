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
	a.NotPanics(func() {
		sqlParse(newLexer(testStandardSelectStmt + ";"))
	})
	a.False(parseResult.extended)
	a.Equal([]string{"employee.age", "last_name", "salary"},
		parseResult.fields)
	a.Equal([]string{"employee"}, parseResult.tables)
	a.Equal("100", parseResult.limit)
	a.Equal(AND, parseResult.where.sexp[0].typ)
	a.Equal('<', rune(parseResult.where.sexp[1].sexp[0].typ))
	a.Equal('=', rune(parseResult.where.sexp[2].sexp[0].typ))
	a.Equal(`employee.age % 10 < (salary / 10000) AND `+
		`strings.Upper(last_name) = "WANG"`,
		parseResult.where.String())
}

func TestTrainParser(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		sqlParse(newLexer(trainSelect))
	})
	a.True(parseResult.extended)
	a.True(parseResult.train)
	a.Equal("DNNClassifier", parseResult.estimator)
	a.Equal("[10, 20]", parseResult.attrs["hidden_units"].String())
	a.Equal("3", parseResult.attrs["n_classes"].String())
	a.Equal(`employee.name`,
		parseResult.columns[0].String())
	a.Equal(`bucketize(last_name, 1000)`,
		parseResult.columns[1].String())
	a.Equal(
		`cross(embedding(emplyoee.name), bucketize(last_name, 1000))`,
		parseResult.columns[2].String())
	a.Equal("employee.salary", parseResult.label)
	a.Equal("my_dnn_model", parseResult.save)
}

func TestPredictParser(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		sqlParse(newLexer(predictSelect))
	})
	a.True(parseResult.extended)
	a.False(parseResult.train)
	a.Equal("my_dnn_model", parseResult.model)
	a.Equal("db.table.field", parseResult.into)
}

func TestSelectStarAndPrint(t *testing.T) {
	a := assert.New(t)
	a.NotPanics(func() {
		sqlParse(newLexer(`SELECT *, b FROM a LIMIT 10;`))
	})
	a.Equal(2, len(parseResult.fields))
	a.Equal("*", parseResult.fields[0])
	a.False(parseResult.extended)
	a.False(parseResult.train)
	a.Equal("SELECT *, b\nFROM a\nLIMIT 10;", parseResult.standardSelect.String())
}
