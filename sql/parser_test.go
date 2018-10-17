package sql

import (
	"bytes"
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

	var buf bytes.Buffer
	parseResult.where.print(&buf)
	assert.Equal(t,
		`employee.age % 10 < (salary / 10000) AND strings.Upper(last_name) = "WANG"`,
		buf.String())
}
