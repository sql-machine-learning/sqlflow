package tidb

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pingcap/parser"
	_ "github.com/pingcap/tidb/types/parser_driver"
)

// Parse calls TiDB's parser to parse a statement sql.  It returns
// <nil,-1> if TiDB parser accepts the statement, or <nil,pos> if TiDB
// doesn't accept but returns a `near "..."` in the error message, or
// <err,-1> if the error messages doens't contain near.
func Parse(sql string) (err error, idx int) {
	p := parser.New()
	_, _, err = p.Parse(sql, "", "")

	if err != nil {
		re := regexp.MustCompile(`.* near "([^"]+)".*`)
		matched := re.FindAllStringSubmatch(err.Error(), -1)

		if len(matched) != 1 || len(matched[0]) != 2 {
			return fmt.Errorf("Cannot match near in %q", err), -1
		}
		return nil, strings.Index(sql, matched[0][1])
	}
	return nil, -1
}
