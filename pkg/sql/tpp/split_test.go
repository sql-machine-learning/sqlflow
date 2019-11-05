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

package parser

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	a := assert.New(t)

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
		s, err := split(sql)
		a.NoError(err)
		a.Equal(1, len(s))
		a.Equal(sql, s[0])
	}

	{ // several standard SQL statements with comments
		sqls := strings.Join(selectCases, `;`) + `;`
		s, err := split(sqls)
		a.NoError(err)
		a.Equal(len(selectCases), len(s))
		for i := range s {
			a.Equal(selectCases[i]+`;`, s[i])
		}
	}

	// two SQL statements, the first one is extendedSQL
	for _, sql := range selectCases {
		sqls := fmt.Sprintf(`%s to train;%s;`, sql, sql)
		s, err := split(sqls)
		a.NoError(err)
		a.Equal(2, len(s))
		a.Equal(sql+` to train;`, s[0])
		a.Equal(sql+`;`, s[1])
	}

	// two SQL statements, the second one is extendedSQL
	for _, sql := range selectCases {
		sqls := fmt.Sprintf(`%s;%s to train;`, sql, sql)
		s, err := split(sqls)
		a.NoError(err)
		a.Equal(2, len(s))
		a.Equal(sql+`;`, s[0])
		a.Equal(sql+` to train;`, s[1])
	}

	// three SQL statements, the second one is extendedSQL
	for _, sql := range selectCases {
		sqls := fmt.Sprintf(`%s;%s to train;%s;`, sql, sql, sql)
		s, err := split(sqls)
		a.NoError(err)
		a.Equal(3, len(s))
		a.Equal(sql+`;`, s[0])
		a.Equal(sql+` to train;`, s[1])
		a.Equal(sql+`;`, s[2])
	}

	{ // two SQL statements, the first standard SQL has an error.
		sql := `select select 1; select 1 to train;`
		s, err := split(sql)
		a.EqualError(err, `line 1 column 13 near "select 1; select 1 to train;" `)
		a.Equal(0, len(s))
	}

	// two SQL statements, the second standard SQL has an error.
	for _, sql := range selectCases {
		sqls := fmt.Sprintf(`%s to train; select select 1;`, sql)
		s, err := split(sqls)
		a.EqualError(err, `line 1 column 14 near "select 1;" `)
		a.Equal(0, len(s))
	}

	{ // non select statement before to train
		sql := `describe table to train;`
		s, err := split(sql)
		a.EqualError(err, `line 1 column 14 near "table to train;" `)
		a.Equal(0, len(s))
	}
}
