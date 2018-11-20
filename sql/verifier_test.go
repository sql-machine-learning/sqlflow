package sql

import (
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
)

var (
	testConfig *mysql.Config
)

func init() {
	testConfig = &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}
}

func TestSanityCheck(t *testing.T) {
	assert := assert.New(t)
	assert.NotPanics(func() {
		sqlParse(newLexer(`SELECT * FROM churn.churn LIMIT 10;`))
	})
	assert.Nil(sanityCheck(&parseResult, testConfig),
		"Make sure you are running the MySQL server in example/churn.")
}

func TestDescribeTables(t *testing.T) {
	assert := assert.New(t)
	assert.NotPanics(func() {
		sqlParse(newLexer(`SELECT * FROM churn.churn LIMIT 10;`))
	})
	_, e := describeTables(&parseResult, testConfig)
	assert.Nil(e,
		"Make sure you are running the MySQL server in example/churn.")
}
