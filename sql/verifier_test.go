package sql

import (
	"database/sql"
	"log"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
)

var (
	testCfg *mysql.Config
	testDB  *sql.DB
)

func init() {
	testCfg = &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}
	db, e := sql.Open("mysql", testCfg.FormatDSN())
	if e != nil {
		log.Panicf("verify cannot connect to MySQL: %q", e)
	}
	testDB = db
}

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
	assert.Equal("varchar(255)", fts["Churn"]["churn.churn"])
	assert.Equal("varchar(255)", fts["Partner"]["churn.churn"])
}
