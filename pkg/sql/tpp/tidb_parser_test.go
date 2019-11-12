// Copyright 2019 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tpp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTiDBParseAndSplit(t *testing.T) {
	a := assert.New(t)
	tiDBInit()

	selectCases := []string{
		`select 1`,
		`select * from my_table`,
		`-- this is a comment
select
1`,
		`SELECT
    customerNumber,
    checkNumber,
    amount
FROM
    payments
WHERE
    amount = (SELECT MAX(amount) FROM payments)`,
		`SELECT
    orderNumber,
    SUM(priceEach * quantityOrdered) total
FROM
    orderdetails
        INNER JOIN
    orders USING (orderNumber)
GROUP BY orderNumber
HAVING SUM(priceEach * quantityOrdered) > 60000`,
		`SELECT
    customerNumber,
    customerName
FROM
    customers
WHERE
    EXISTS( SELECT
            orderNumber, SUM(priceEach * quantityOrdered)
        FROM
            orderdetails
                INNER JOIN
            orders USING (orderNumber)
        WHERE
            customerNumber = customers.customerNumber
        GROUP BY orderNumber
        HAVING SUM(priceEach * quantityOrdered) > 60000)`,
	}

	// one standard SQL statement
	for _, sql := range selectCases {
		s, idx, err := tiDBParseAndSplit(sql)
		a.NoError(err)
		a.Equal(-1, idx)
		a.Equal(1, len(s))
		a.Equal(sql, s[0])
	}

	{ // several standard SQL statements with comments
		sqls := strings.Join(selectCases, `;`) + `;`
		s, idx, err := tiDBParseAndSplit(sqls)
		a.NoError(err)
		a.Equal(-1, idx)
		a.Equal(len(selectCases), len(s))
		for i := range s {
			a.Equal(selectCases[i]+`;`, s[i])
		}
	}

	// two SQL statements, the first one is extendedSQL
	for _, sql := range selectCases {
		sqls := fmt.Sprintf(`%s to train;%s;`, sql, sql)
		s, idx, err := tiDBParseAndSplit(sqls)
		a.NoError(err)
		a.Equal(len(sql)+1, idx)
		a.Equal(1, len(s))
		a.Equal(sql+" ", s[0])
	}

	// two SQL statements, the second one is extendedSQL
	for _, sql := range selectCases {
		sqls := fmt.Sprintf(`%s;%s to train;`, sql, sql)
		s, idx, err := tiDBParseAndSplit(sqls)
		a.NoError(err)
		a.Equal(len(sql)+1+len(sql)+1, idx)
		a.Equal(2, len(s))
		a.Equal(sql+`;`, s[0])
		a.Equal(sql+` `, s[1])
	}

	// three SQL statements, the second one is extendedSQL
	for _, sql := range selectCases {
		sqls := fmt.Sprintf(`%s;%s to train;%s;`, sql, sql, sql)
		s, idx, err := tiDBParseAndSplit(sqls)
		a.NoError(err)
		a.Equal(len(sql)+1+len(sql)+1, idx)
		a.Equal(2, len(s))
		a.Equal(sql+`;`, s[0])
		a.Equal(sql+` `, s[1])
	}

	{ // two SQL statements, the first standard SQL has an error.
		sql := `select select 1; select 1 to train;`
		s, idx, err := tiDBParseAndSplit(sql)
		a.Nil(s)
		a.Equal(-1, idx)
		a.EqualError(err, `line 1 column 13 near "select 1; select 1 to train;" `)
	}

	// two SQL statements, the second standard SQL has an error.
	for _, sql := range selectCases {
		sqls := fmt.Sprintf(`%s to train; select select 1;`, sql)
		s, idx, err := tiDBParseAndSplit(sqls)
		a.NoError(err)
		a.Equal(len(sql)+1, idx)
		a.Equal(1, len(s))
		a.Equal(sql+` `, s[0])
	}

	{ // non select statement before to train
		sql := `describe table to train;`
		s, idx, err := tiDBParseAndSplit(sql)
		a.EqualError(err, `line 1 column 14 near "table to train;" `)
		a.Equal(0, len(s))
		a.Equal(-1, idx)
	}
}

func TestTiDBParseAndSplitIdx(t *testing.T) {
	a := assert.New(t)
	var (
		i int
		e error
	)

	tiDBInit()

	_, i, e = tiDBParseAndSplit("SELECTED a FROM t1") // SELECTED => SELECT
	a.Equal(-1, i)
	a.Error(e)

	_, i, e = tiDBParseAndSplit("SELECT * FROM t1 TO TRAIN DNNClassifier")
	a.Equal(17, i)
	a.NoError(e)

	_, i, e = tiDBParseAndSplit("SELECT * FROM t1 TO TO TRAIN DNNClassifier")
	a.Equal(17, i)
	a.NoError(e)

	_, i, e = tiDBParseAndSplit("SELECT * FROM t1 t2 TO TRAIN DNNClassifier") // t2 is an alias of t1
	a.Equal(20, i)
	a.NoError(e)

	_, i, e = tiDBParseAndSplit("SELECT * FROM t1 t2, t3 TO TRAIN DNNClassifier") // t2 is an alias of t1
	a.Equal(24, i)
	a.NoError(e)

	_, i, e = tiDBParseAndSplit("SELECT * FROM t1 t2, t3 t4 TO TRAIN DNNClassifier") // t2 and t4 are aliases.
	a.Equal(27, i)
	a.NoError(e)

	_, i, e = tiDBParseAndSplit("SELECT * FROM (SELECT * FROM t1)")
	a.Equal(-1, i)
	a.Error(e) // TiDB parser and MySQL require an alias name after the nested SELECT.

	_, i, e = tiDBParseAndSplit("SELECT * FROM (SELECT * FROM t1) t2")
	a.Equal(-1, i)
	a.NoError(e)

	_, i, e = tiDBParseAndSplit("SELECT * FROM (SELECT * FROM t1) t2 TO TRAIN DNNClassifier")
	a.Equal(36, i)
	a.NoError(e)
}
