package sqlfile

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
)

var (
	testCfg *mysql.Config
	testDB  *sql.DB
)

func TestCreateAndDropTable(t *testing.T) {
	fn := fmt.Sprintf("sqlfile.unitest-%d", rand.Int())
	assert.NotNil(t, createTable(testDB, fn))
	assert.NotNil(t, dropTable, fn)
}

func TestMain(m *testing.M) {
	testCfg = &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}
	db, e := sql.Open("mysql", testCfg.FormatDSN())
	if e != nil {
		log.Panicf("TestMain cannot connect to MySQL: %q.\n"+
			"Please run MySQL server as in example/churn/README.md.", e)
	}
	testDB = db

	defer testDB.Close()
	os.Exit(m.Run())
}
