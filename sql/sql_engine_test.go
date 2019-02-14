package sql

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestSQLOpen(t *testing.T) {
	n := fmt.Sprintf("%d%d", time.UnixTime(), os.Getpid())
	db, e := sql.Open("sqlite3", n)
	assert.NoError(t, e)
	assert.NotNil(db)
}
