//go:generate protoc CalciteParser.proto --go_out=plugins=grpc:.
//
// This package is a gRPC client that implements CalciteParser.proto.
// The server implementation is in CalciteParserServer.java.
package calcite

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalciteParser(t *testing.T) {
	if addr := os.Getenv("SQLFLOW_CALCITE_PARSER"); len(addr) > 0 {
		Init(addr)
	} else {
		t.Logf("Cannot connect to CalciteParserServer; skip TestCalciteParser")
		return
	}

	var (
		i int
		e error
		a = assert.New(t)
	)

	i, e = Parse("SELECTED a FROM t1") // SELECTED => SELECT
	a.Equal(0, i)
	a.Error(e)

	i, e = Parse("SELECT a FROM t1") // (i,e)==(-1,nil) indicates legal native SQL syntax.
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT a, b FROM t1")
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT a b FROM t1") // Calcite doesn't need ',' between fields.
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT a b c FROM t1") // Calcite doesn't accept three fields without ','.
	a.Equal(11, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1")
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1, t2")
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1 t2") // Calcite doesn't need ',' between two tables.
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT * FROM t1 t2 t3") // Calcite doesn't accept three fields without ','.
	a.Equal(20, i)
	a.NoError(e)

	i, e = Parse("SELECT a FROM t1 WHERE a IN (SELECT a FROM t2 WHERE Quantity > 100)")
	a.Equal(-1, i)
	a.NoError(e)

	i, e = Parse("SELECT a FROM t1 WHERE a IN (SELECT a FROM t2 WHERE Quantity > 100) TRAIN DNNClassifier")
	a.Equal(68, i) // before TRAIN
	a.NoError(e)

	i, e = Parse("SELECT a FROM t1 WHERE a IN (SELECT a FROM t2 WHERE Quantity > 100) Predict DNNClassifier")
	a.Equal(68, i) // before Predict
	a.NoError(e)

	i, e = Parse("SELECT a FROM t1 PREDICT DNNClassifier")
	a.Equal(25, i) // This looks a bug of Calcite parser. See cases ahead.
	a.NoError(e)
}
