package sqlfile

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
)

var (
	testCfg *mysql.Config
	testDB  *sql.DB
)

func TestCreateHasDropTable(t *testing.T) {
	assert := assert.New(t)

	fn := fmt.Sprintf("sqlfile.unitest%d", rand.Int())
	assert.NoError(createTable(testDB, fn))
	has, e := HasTable(testDB, fn)
	assert.NoError(e)
	assert.True(has)
	assert.NoError(DropTable(testDB, fn))
}

func TestWriterCreate(t *testing.T) {
	assert := assert.New(t)

	fn := fmt.Sprintf("sqlfile.unitest%d", rand.Int())
	w, e := Create(testDB, fn)
	assert.NoError(e)
	assert.NotNil(w)
	defer w.Close()

	has, e1 := HasTable(testDB, fn)
	assert.NoError(e1)
	assert.True(has)

	assert.NoError(DropTable(testDB, fn))
}

func TestWriteAndRead(t *testing.T) {
	assert := assert.New(t)

	fn := fmt.Sprintf("sqlfile.unitest%d", rand.Int())

	w, e := Create(testDB, fn)
	assert.NoError(e)
	assert.NotNil(w)

	// A small output.
	buf := []byte("\n\n\n")
	n, e := w.Write(buf)
	assert.NoError(e)
	assert.Equal(len(buf), n)

	// A big output.
	buf = make([]byte, kBufSize+1)
	for i := range buf {
		buf[i] = 'x'
	}
	n, e = w.Write(buf)
	assert.NoError(e)
	assert.Equal(len(buf), n)

	assert.NoError(w.Close())

	r, e := Open(testDB, fn)
	assert.NoError(e)
	assert.NotNil(r)

	// A small read
	buf = make([]byte, 2)
	n, e = r.Read(buf)
	assert.NoError(e)
	assert.Equal(2, n)
	assert.Equal(2, strings.Count(string(buf), "\n"))

	// A big read of rest
	buf = make([]byte, kBufSize*2)
	n, e = r.Read(buf)
	assert.Equal(io.EOF, e)
	assert.Equal(kBufSize+2, n)
	assert.Equal(1, strings.Count(string(buf), "\n"))
	assert.Equal(kBufSize+1, strings.Count(string(buf), "x"))

	// Another big read
	n, e = r.Read(buf)
	assert.Equal(io.EOF, e)
	assert.Equal(0, n)
	assert.NoError(r.Close())

	assert.NoError(DropTable(testDB, fn))
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
