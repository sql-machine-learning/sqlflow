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
}
