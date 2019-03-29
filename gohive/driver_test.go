package gohive

import "testing"
import "log"
import "database/sql"


func TestOpenConnection(t *testing.T) {
        db, err := sql.Open("hive", "127.0.0.1:10000")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
}

func TestQuery(t *testing.T) {
        db, _ := sql.Open("hive", "127.0.0.1:10000/churn")
        var (
	        customerid string 
	        gender string 
        )
        rows, err := db.Query("SELECT customerID, gender FROM churn.train")
        if err != nil {
	        log.Fatal(err)
        }
        defer db.Close()
        defer rows.Close()
        for rows.Next() {
	        err := rows.Scan(&customerid, &gender)
	        if err != nil {
		        log.Fatal(err)
	        }
	        log.Println(customerid, gender)
        }
        err = rows.Err()
        if err != nil {
	        log.Fatal(err)
        }
}
