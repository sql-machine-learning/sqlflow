package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDryRunSelect(t *testing.T) {
	assert := assert.New(t)
	assert.NotPanics(func() {
		sqlParse(newLexer(`SELECT * FROM churn.churn LIMIT 10;`))
	})
	assert.Nil(dryRunSelect(&parseResult, testDB),
		"Make sure you are running the MySQL server in example/churn.")
}

func TestDescribeTables(t *testing.T) {
	assert := assert.New(t)

	assert.NotPanics(func() {
		sqlParse(newLexer(`SELECT * FROM churn.churn LIMIT 10;`))
	})
	fts, e := describeTables(&parseResult, testDB)
	assert.Nil(e,
		"Make sure you are running the MySQL server in example/churn.")
	assert.Equal(21, len(fts))

	assert.NotPanics(func() {
		sqlParse(newLexer(`SELECT Churn, churn.churn.Partner FROM churn.churn LIMIT 10;`))
	})
	fts, e = describeTables(&parseResult, testDB)
	assert.Nil(e,
		"Make sure you are running the MySQL server in example/churn.")
	assert.Equal(2, len(fts))
	assert.Equal("varchar(255)", fts["Churn"]["churn.churn"])
	assert.Equal("varchar(255)", fts["Partner"]["churn.churn"])
}

func TestIndexSelectFields(t *testing.T) {
	assert := assert.New(t)

	assert.NotPanics(func() {
		sqlParse(newLexer(`SELECT * FROM churn.churn LIMIT 10;`))
	})
	f := indexSelectFields(&parseResult)
	assert.Equal(0, len(f))

	assert.NotPanics(func() {
		sqlParse(newLexer(`SELECT f FROM churn.churn LIMIT 10;`))
	})
	f = indexSelectFields(&parseResult)
	assert.Equal(1, len(f))
	assert.Equal(map[string]string{}, f["f"])

	assert.NotPanics(func() {
		sqlParse(newLexer(`SELECT t1.f, t2.f, g FROM churn.churn LIMIT 10;`))
	})
	f = indexSelectFields(&parseResult)
	assert.Equal(2, len(f))
	assert.Equal(map[string]string{}, f["g"])
	assert.Equal("", f["f"]["t1"])
	assert.Equal("", f["f"]["t2"])
}

func TestVerify(t *testing.T) {
	assert := assert.New(t)
	assert.NotPanics(func() {
		sqlParse(newLexer(`SELECT Churn, churn.churn.Partner FROM churn.churn LIMIT 10;`))
	})
	fts, e := verify(&parseResult, testCfg)
	assert.Nil(e,
		"Make sure you are running the MySQL server in example/churn.")
	assert.Equal(2, len(fts))
	typ, ok := fts.get("Churn")
	assert.Equal(true, ok)
	assert.Equal("varchar(255)", typ)

	typ, ok = fts.get("churn.churn.Partner")
	assert.Equal(true, ok)
	assert.Equal("varchar(255)", typ)

	typ, ok = fts.get("churn.churn.gender")
	assert.Equal(false, ok)

	typ, ok = fts.get("gender")
	assert.Equal(false, ok)
}
