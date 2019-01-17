// To checkout this program:
//
//   export GOPATH=$HOME/go
//   go get -u github.com/wangkuiyi/sqlflow/cmd/sqlparser
//
// To build after modifying the source code:
//
//    go install github.com/wangkuiyi/sqlflow/cmd/sqlparser
//
// To run this program:
//
//    cat testdata/train_test.sql | $GOPATH/bin/sqlparser
//
package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/wangkuiyi/sqlflow/sql"
)

func main() {
	if s, e := ioutil.ReadAll(os.Stdin); e == nil {
		jsonStr, e := sql.ParseToJSON(string(s))
		if e != nil {
			fmt.Println(e)
		} else {
			fmt.Println(jsonStr)
		}
	}
}
