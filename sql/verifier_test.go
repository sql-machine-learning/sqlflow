package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDryRunSelect(t *testing.T) {
	a := assert.New(t)
	var pr extendedSelect
	a.NotPanics(func() {
		pr = parseSQL(`SELECT * FROM churn.churn LIMIT 10;`)
	})
	a.Nil(dryRunSelect(&pr, testDB))
}

func TestDescribeTables(t *testing.T) {
	a := assert.New(t)

	var pr extendedSelect
	a.NotPanics(func() {
		pr = parseSQL(`SELECT * FROM churn.churn LIMIT 10;`)
	})
	fts, e := describeTables(&pr, testDB)
	a.NoError(e)
	a.Equal(21, len(fts))

	a.NotPanics(func() {
		pr = parseSQL(`SELECT Churn, churn.churn.Partner FROM churn.churn LIMIT 10;`)
	})
	fts, e = describeTables(&pr, testDB)
	a.NoError(e)
	a.Equal(2, len(fts))
	a.Equal("varchar(255)", fts["Churn"]["churn.churn"])
	a.Equal("varchar(255)", fts["Partner"]["churn.churn"])
}

func TestIndexSelectFields(t *testing.T) {
	a := assert.New(t)

	var pr extendedSelect
	a.NotPanics(func() {
		pr = parseSQL(`SELECT * FROM churn.churn LIMIT 10;`)
	})
	f := indexSelectFields(&pr)
	a.Equal(0, len(f))

	a.NotPanics(func() {
		pr = parseSQL(`SELECT f FROM churn.churn LIMIT 10;`)
	})
	f = indexSelectFields(&pr)
	a.Equal(1, len(f))
	a.Equal(map[string]string{}, f["f"])

	a.NotPanics(func() {
		pr = parseSQL(`SELECT t1.f, t2.f, g FROM churn.churn LIMIT 10;`)
	})
	f = indexSelectFields(&pr)
	a.Equal(2, len(f))
	a.Equal(map[string]string{}, f["g"])
	a.Equal("", f["f"]["t1"])
	a.Equal("", f["f"]["t2"])
}

func TestVerify(t *testing.T) {
	a := assert.New(t)
	var pr extendedSelect
	a.NotPanics(func() {
		pr = parseSQL(`SELECT Churn, churn.churn.Partner FROM churn.churn LIMIT 10;`)
	})
	fts, e := verify(&pr, testCfg)
	a.NoError(e)
	a.Equal(2, len(fts))
	typ, ok := fts.get("Churn")
	a.Equal(true, ok)
	a.Equal("varchar(255)", typ)

	typ, ok = fts.get("churn.churn.Partner")
	a.Equal(true, ok)
	a.Equal("varchar(255)", typ)

	_, ok = fts.get("churn.churn.gender")
	a.Equal(false, ok)

	_, ok = fts.get("gender")
	a.Equal(false, ok)
}
