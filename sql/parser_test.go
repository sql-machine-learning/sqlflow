package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParser(t *testing.T) {
	sel := `
SELECT employee.age, salary 
FROM   employee
LIMIT  100
WHERE  employee.age % 10 < (salary / 10000)
;
`
	assert.NotPanics(t, func() {
		p := sqlNewParser()
		p.Parse(newLexer(sel))
	})
}
