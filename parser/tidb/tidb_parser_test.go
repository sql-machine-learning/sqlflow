package tidb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTiDBParser(t *testing.T) {
	a := assert.New(t)

	i, e := Parse("select * frm tbl") // frm->from
	a.NoError(e)
	a.Equal(9, i)

	i, e = Parse("select * from tbl where id = 1")
	a.NoError(e)
	a.Equal(-1, i)

	i, e = Parse("select * from tbl where id = 1 TRAIN DNNClassifier WITH learning_rate=0.1")
	a.NoError(e)
	a.Equal(31, i)

	i, e = Parse("select * from tbl where id = 1 predict tbl.predicted using my_model")
	a.NoError(e)
	a.Equal(31, i)

	i, e = Parse("select * from tbl where id = 1 predict tbl.predicted uses my_model") // uses->using
	a.NoError(e)
	a.Equal(31, i)
}
