package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/wangkuiyi/sqlflow/sql"
)

func main() {
	if s, e := ioutil.ReadAll(os.Stdin); e == nil {
		fmt.Println(sql.Parse(string(s)))
	}
}
