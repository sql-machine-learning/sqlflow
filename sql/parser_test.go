package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParser(t *testing.T) {
	assert := assert.New(t)

	sel := `
SELECT employee.age, last_name, salary 
FROM   employee
LIMIT  100
WHERE  
  employee.age % 10 < (salary / 10000) 
  AND 
  strings.Upper(last_name) = "WANG"
TRAIN DNNClassifier
WITH 
  n_classes = 3,
  hidden_units = [10, 20]
COLUMN
  employee.name,
  bucketize(last_name, 1000), 
  cross(embedding(emplyoee.name), bucketize(last_name, 1000))
INTO
  my_dnn_model
;
`
	assert.NotPanics(func() {
		sqlParse(newLexer(sel))
	})
	assert.Equal([]string{"employee.age", "last_name", "salary"},
		parseResult.fields)
	assert.Equal([]string{"employee"}, parseResult.tables)
	assert.Equal("100", parseResult.limit)
	assert.Equal(AND, parseResult.where.sexp[0].typ)
	assert.Equal('<', rune(parseResult.where.sexp[1].sexp[0].typ))
	assert.Equal('=', rune(parseResult.where.sexp[2].sexp[0].typ))
	assert.Equal("DNNClassifier", parseResult.estimator)

	assert.Equal(`employee.age % 10 < (salary / 10000) AND `+
		`strings.Upper(last_name) = "WANG"`,
		parseResult.where.String())

	assert.Equal("[10, 20]", parseResult.attrs["hidden_units"].String())
	assert.Equal("3", parseResult.attrs["n_classes"].String())

	assert.Equal(`employee.name`,
		parseResult.columns[0].String())
	assert.Equal(`bucketize(last_name, 1000)`,
		parseResult.columns[1].String())
	assert.Equal(
		`cross(embedding(emplyoee.name), bucketize(last_name, 1000))`,
		parseResult.columns[2].String())

	assert.Equal("my_dnn_model", parseResult.into)
}
