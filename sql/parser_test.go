package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParser(t *testing.T) {
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
;
`
	assert.NotPanics(t, func() {
		sqlParse(newLexer(sel))
	})
	assert.Equal(t, []string{"employee.age", "last_name", "salary"},
		parseResult.fields)
	assert.Equal(t, []string{"employee"}, parseResult.tables)
	assert.Equal(t, "100", parseResult.limit)
	assert.Equal(t, AND, parseResult.where.sexp[0].typ)
	assert.Equal(t, '<', rune(parseResult.where.sexp[1].sexp[0].typ))
	assert.Equal(t, '=', rune(parseResult.where.sexp[2].sexp[0].typ))
	assert.Equal(t, "DNNClassifier", parseResult.estimator)

	assert.Equal(t,
		`employee.age % 10 < (salary / 10000) AND strings.Upper(last_name) = "WANG"`,
		parseResult.where.String())

	assert.Equal(t, "[10, 20]", parseResult.attrs["hidden_units"].String())
	assert.Equal(t, "3", parseResult.attrs["n_classes"].String())
}
