package tidb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTiDBParser(t *testing.T) {
	a := assert.New(t)

	e, i := Parse("select * from tbl where id = 1")
	a.NoError(e)
	a.Equal(i, -1)

	e, i = Parse("select * from tbl where id = 1 TRAIN DNNClassifier WITH learning_rate=0.1")
	a.NoError(e)
	a.Equal(i, 31)

	e, i = Parse("select * from tbl where id = 1 predict tbl.predicted using my_model")
	a.NoError(e)
	a.Equal(i, 31)
}
