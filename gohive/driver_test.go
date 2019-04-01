package gohive

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenConnection(t *testing.T) {
	db, err := sql.Open("hive", "127.0.0.1:10000")
	assert.Nil(t, err)
	defer db.Close()
}

func TestQuery(t *testing.T) {
	db, _ := sql.Open("hive", "127.0.0.1:10000/churn")
	rows, err := db.Query("SELECT customerID, gender FROM churn.train")
	assert.Nil(t, err)
	defer db.Close()
	defer rows.Close()

	n := 0
	customerid := ""
	gender := ""
	for rows.Next() {
		err := rows.Scan(&customerid, &gender)
		assert.Nil(t, err)
		n++
	}
	assert.Nil(t, rows.Err())
	assert.Equal(t, 82, n) // The imported data size is 82.
}
