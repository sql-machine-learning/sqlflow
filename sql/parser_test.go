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
  last_name = "Wang"
;
`
	assert.NotPanics(t, func() {
		sqlParse(newLexer(sel))
	})
	assert.Equal(t, []string{"employee.age", "last_name", "salary"},
		parseResult.fields)
	assert.Equal(t, []string{"employee"}, parseResult.tables)
	assert.Equal(t, "100", parseResult.limit)
	assert.Equal(t, AND, parseResult.where.typ)
	assert.Equal(t, '<', rune(parseResult.where.oprd[0].typ))
	assert.Equal(t, '=', rune(parseResult.where.oprd[1].typ))
}
