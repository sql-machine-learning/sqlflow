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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func isJavaParser(typ string) bool {
	return typ == "hive" || typ == "calcite"
}

func TestParse(t *testing.T) {
	a := assert.New(t)

	extendedSQL := `to predict a using b`

	selectCases := []string{
		`select 1`,
		`select * from my_table
`,
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

	for _, driver := range []string{"mysql", "hive", "calcite"} {
		// one standard SQL statement
		for _, sql := range selectCases {
			s, err := parse(driver, sql+";")
			a.NoError(err)
			a.Equal(1, len(s))
			a.Nil(s[0].extended)
			if isJavaParser(driver) {
				a.Equal(sql, s[0].standard)
			} else {
				a.Equal(sql+`;`, s[0].standard)
			}
		}

		{ // several standard SQL statements with comments
			sqls := strings.Join(selectCases, `;`) + `;`
			s, err := parse(driver, sqls)
			a.NoError(err)
			a.Equal(len(selectCases), len(s))
			for i := range s {
				a.Nil(s[i].extended)
				if isJavaParser(driver) {
					a.Equal(selectCases[i], s[i].standard)
				} else {
					a.Equal(selectCases[i]+`;`, s[i].standard)
				}
			}
		}

		// two SQL statements, the first one is extendedSQL
		for _, sql := range selectCases {
			sqls := fmt.Sprintf(`%s %s;%s;`, sql, extendedSQL, sql)
			s, err := parse(driver, sqls)
			a.NoError(err)
			a.Equal(2, len(s))

			a.NotNil(s[0].extended)
			a.Equal(sql+` `, s[0].standard)
			a.Equal(fmt.Sprintf(`%s %s;`, sql, extendedSQL), s[0].original)

			a.Nil(s[1].extended)
			if isJavaParser(driver) {
				a.Equal(sql, s[1].standard)
			} else {
				a.Equal(sql+`;`, s[1].standard)
			}
		}

		// two SQL statements, the second one is extendedSQL
		for _, sql := range selectCases {
			sqls := fmt.Sprintf(`%s;%s %s;`, sql, sql, extendedSQL)
			s, err := parse(driver, sqls)
			a.NoError(err)
			a.Equal(2, len(s))
			a.Nil(s[0].extended)
			a.NotNil(s[1].extended)
			if isJavaParser(driver) {
				a.Equal(sql, s[0].standard)
			} else {
				a.Equal(sql+`;`, s[0].standard)
			}
			a.Equal(sql+` `, s[1].standard)
			a.Equal(fmt.Sprintf(`%s %s;`, sql, extendedSQL), s[1].original)
		}

		// three SQL statements, the second one is extendedSQL
		for _, sql := range selectCases {
			sqls := fmt.Sprintf(`%s;%s %s;%s;`, sql, sql, extendedSQL, sql)
			s, err := parse(driver, sqls)
			a.NoError(err)
			a.Equal(3, len(s))

			a.Nil(s[0].extended)
			a.NotNil(s[1].extended)
			a.Nil(s[2].extended)

			if isJavaParser(driver) {
				a.Equal(sql, s[0].standard)
				a.Equal(sql, s[2].standard)
			} else {
				a.Equal(sql+`;`, s[0].standard)
				a.Equal(sql+`;`, s[2].standard)
			}

			a.Equal(sql+` `, s[1].standard)
			a.Equal(fmt.Sprintf(`%s %s;`, sql, extendedSQL), s[1].original)
		}

		{ // two SQL statements, the first standard SQL has an error.
			sql := `select select 1; select 1 to train;`
			s, err := parse(driver, sql)
			a.NotNil(err)
			a.Equal(0, len(s))
		}

		// two SQL statements, the second standard SQL has an error.
		for _, sql := range selectCases {
			sqls := fmt.Sprintf(`%s %s; select select 1;`, sql, extendedSQL)
			s, err := parse(driver, sqls)
			a.NotNil(err)
			a.Equal(0, len(s))
		}

		{ // non select statement before to train
			sql := fmt.Sprintf(`describe table %s;`, extendedSQL)
			s, err := parse(driver, sql)
			a.NotNil(err)
			a.Equal(0, len(s))
		}
	}
}
