// Copyright 2020 The SQLFlow Authors. All rights reserved.
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
	"sqlflow.org/sqlflow/go/parser/external"
)

func isJavaParser(dbms string) bool {
	return dbms == "hive" || dbms == "calcite"
}

func TestParseWithMySQL(t *testing.T) {
	commonTestCases("mysql", assert.New(t))
}

func TestParseWithHive(t *testing.T) {
	commonTestCases("hive", assert.New(t))
}

func TestParseWithCalcite(t *testing.T) {
	commonTestCases("calcite", assert.New(t))
}

func commonTestCases(dbms string, a *assert.Assertions) {
	extendedSQL := `to predict a using b`

	// one standard SQL statement
	for _, sql := range external.SelectCases {
		s, err := Parse(dbms, sql+";")
		a.NoError(err)
		a.Equal(1, len(s))
		a.False(s[0].IsExtendedSyntax())
		if isJavaParser(dbms) {
			a.Equal(sql, s[0].Original)
		} else {
			a.Equal(sql+`;`, s[0].Original)
		}
	}

	{ // several standard SQL statements with comments
		sqls := strings.Join(external.SelectCases, `;`) + `;`
		s, err := Parse(dbms, sqls)
		a.NoError(err)
		a.Equal(len(external.SelectCases), len(s))
		for i := range s {
			a.False(s[i].IsExtendedSyntax())
			if isJavaParser(dbms) {
				a.Equal(external.SelectCases[i], s[i].Original)
			} else {
				a.Equal(external.SelectCases[i]+`;`, s[i].Original)
			}
		}
	}

	// two SQL statements, the first one is extendedSQL
	for _, sql := range external.SelectCases {
		sqls := fmt.Sprintf(`%s %s;%s;`, sql, extendedSQL, sql)
		s, err := Parse(dbms, sqls)
		a.NoError(err)
		a.Equal(2, len(s))

		a.True(s[0].IsExtendedSyntax())
		a.Equal(sql+` `, s[0].StandardSelect.String())
		a.Equal(fmt.Sprintf(`%s %s;`, sql, extendedSQL), s[0].Original)

		a.False(s[1].IsExtendedSyntax())
		if isJavaParser(dbms) {
			a.Equal(sql, s[1].Original)
		} else {
			a.Equal(sql+`;`, s[1].Original)
		}
	}
	// two SQL statements, the first one is SHOW TRAIN
	{
		extendedSQL := `SHOW TRAIN my_model`
		for _, sql := range external.SelectCases {
			sqls := fmt.Sprintf(`%s;%s;`, extendedSQL, sql)
			s, err := Parse(dbms, sqls)
			a.NoError(err)
			a.Equal(2, len(s))
			a.True(s[0].Extended)
			a.True(s[0].ShowTrain)
			a.Equal("my_model", s[0].ShowTrainClause.ModelName)
		}
		for _, sql := range external.SelectCases {
			sqls := fmt.Sprintf(`%s %s;%s;`, sql, extendedSQL, sql)
			s, err := Parse(dbms, sqls)
			a.Error(err, "select should followed by 'to train/predict/explain'")
			a.Equal(0, len(s))
		}
	}
	// two SQL statements, the second one is extendedSQL
	for _, sql := range external.SelectCases {
		sqls := fmt.Sprintf(`%s;%s %s;`, sql, sql, extendedSQL)
		s, err := Parse(dbms, sqls)
		a.NoError(err)
		a.Equal(2, len(s))
		a.False(s[0].IsExtendedSyntax())
		a.True(s[1].IsExtendedSyntax())
		if isJavaParser(dbms) {
			a.Equal(sql, s[0].Original)
		} else {
			a.Equal(sql+`;`, s[0].Original)
		}
		a.Equal(sql+` `, s[1].StandardSelect.String())
		a.Equal(fmt.Sprintf(`%s %s;`, sql, extendedSQL), s[1].Original)
	}
	// two SQL statements, the second one is SHOW TRAIN
	{
		extendedSQL := `SHOW TRAIN my_model;`
		for _, sql := range external.SelectCases {
			sqls := fmt.Sprintf(`%s;%s`, sql, extendedSQL)
			s, err := Parse(dbms, sqls)
			a.NoError(err)
			a.Equal(2, len(s))
			a.False(s[0].IsExtendedSyntax())
			a.True(s[1].IsExtendedSyntax())
			if isJavaParser(dbms) {
				a.Equal(sql, s[0].Original)
			} else {
				a.Equal(sql+`;`, s[0].Original)
			}
			a.Equal(extendedSQL, s[1].Original)
			a.True(s[1].ShowTrain)
			a.Equal("my_model", s[1].ShowTrainClause.ModelName)
		}
	}

	// three SQL statements, the second one is extendedSQL
	for _, sql := range external.SelectCases {
		sqls := fmt.Sprintf(`%s;%s %s;%s;`, sql, sql, extendedSQL, sql)
		s, err := Parse(dbms, sqls)
		a.NoError(err)
		a.Equal(3, len(s))

		a.False(s[0].IsExtendedSyntax())
		a.True(s[1].IsExtendedSyntax())
		a.False(s[2].IsExtendedSyntax())

		if isJavaParser(dbms) {
			a.Equal(sql, s[0].Original)
			a.Equal(sql, s[2].Original)
		} else {
			a.Equal(sql+`;`, s[0].Original)
			a.Equal(sql+`;`, s[2].Original)
		}

		a.Equal(sql+` `, s[1].StandardSelect.String())
		a.Equal(fmt.Sprintf(`%s %s;`, sql, extendedSQL), s[1].Original)
	}
	// three SQL statements, the second one SHOW TRAIN
	{
		extendedSQL := `SHOW TRAIN my_model`
		for _, sql := range external.SelectCases {
			sqls := fmt.Sprintf(`%s;%s;%s;`, sql, extendedSQL, sql)
			s, err := Parse(dbms, sqls)
			a.NoError(err)
			a.Equal(3, len(s))

			a.False(s[0].IsExtendedSyntax())
			a.True(s[1].IsExtendedSyntax())
			a.False(s[2].IsExtendedSyntax())

			if isJavaParser(dbms) {
				a.Equal(sql, s[0].Original)
				a.Equal(sql, s[2].Original)
			} else {
				a.Equal(sql+`;`, s[0].Original)
				a.Equal(sql+`;`, s[2].Original)
			}
			a.Equal(extendedSQL+";", s[1].Original)
			a.True(s[1].ShowTrain)
			a.Equal("my_model", s[1].ShowTrainClause.ModelName)
		}
	}
	{ // two SQL statements, the first standard SQL has an error.
		sql := `select select 1; select 1 to train;`
		s, err := Parse(dbms, sql)
		a.NotNil(err)
		a.Equal(0, len(s))
	}

	{ // SELECT...UNION...SELECT statement
		sql := `select * from (select 1 limit 1) a union select * from (select 1) b to explain model;`

		s, err := Parse(dbms, sql)
		a.Nil(err)
		a.Equal(1, len(s))
	}

	// two SQL statements, the second standard SQL has an error.
	for _, sql := range external.SelectCases {
		sqls := fmt.Sprintf(`%s %s; select select 1;`, sql, extendedSQL)
		s, err := Parse(dbms, sqls)
		a.NotNil(err)
		a.Equal(0, len(s))
	}

	{ // non select statement before to train
		sql := fmt.Sprintf(`describe table %s;`, extendedSQL)
		s, err := Parse(dbms, sql)
		a.NotNil(err)
		a.Equal(0, len(s))
	}

	{ // multiple statements with multiple lines comments
		sql := `-- TRAIN WITH TF
SELECT * FROM jtest_dev.sqlflow_fraud_detection
TO TRAIN DNNClassifier WITH
    train.batch_size=2048,
    model.batch_norm=True,
    model.hidden_units=[200, 100, 50]
LABEL class
INTO sqlflow_fraud_detection_model;

-- -- TRAIN WITH XGBOOST
-- SELECT * FROM jtest_dev.sqlflow_fraud_detection
-- TO TRAIN XGBoost.gbtree WITH
--     objective="binary:logistic"
-- LABEL class
-- INTO sqlflow_fraud_detection_model;

-- PREDICT WITH TRAINED MODEL
SELECT * FROM jtest_dev.sqlflow_fraud_detection_pred
TO PREDICT jtest_dev.sqlflow_fraud_detection_predict.class
USING sqlflow_fraud_detection_model;

-- EXPLAIN
SELECT * FROM jtest_dev.sqlflow_fraud_detection_pred
TO EXPLAIN sqlflow_fraud_detection_model;

-- SHOW TRAIN
SHOW TRAIN  sqlflow_fraud_detection_model;
`
		s, err := Parse(dbms, sql)
		a.Nil(err)
		a.Equal(4, len(s))
		for _, ss := range s {
			// check parsing on individual statement
			sss, err := Parse(dbms, ss.Original)
			a.Nil(err)
			a.Equal(1, len(sss))
		}
	}
}

func TestParseFirstSQLStatement(t *testing.T) {
	a := assert.New(t)

	{
		pr, idx, e := parseFirstSQLFlowStmt(`to train a with b = c label d into e; select a from b;`)
		a.NotNil(pr)
		a.Equal(len(`to train a with b = c label d into e; `), idx)
		a.NoError(e)
	}

	{
		// corner case: no space between two statements
		pr, idx, e := parseFirstSQLFlowStmt(`to train a with b = c label d into e;select a from b;`)
		a.NotNil(pr)
		a.Equal(len(`to train a with b = c label d into e;`), idx)
		a.NoError(e)
	}

	{
		pr, idx, e := parseFirstSQLFlowStmt(`to train a with b = c label d into e;"`)
		a.NotNil(pr)
		a.Equal(len(`to train a with b = c label d into e;`), idx)
		a.NoError(e)
	}

	{
		pr, idx, e := parseFirstSQLFlowStmt(`to train a with b =?? c label d into e ...`)
		a.Nil(pr)
		a.Equal(-1, idx)
		a.Error(e)
	}

	{
		pr, idx, e := parseFirstSQLFlowStmt(`to train a with b = c label d into e select a from b;`)
		a.Nil(pr)
		a.Equal(-1, idx)
		a.Error(e)
	}
}

func TestParserErrorMessage(t *testing.T) {
	a := assert.New(t)
	pr, idx, e := parseFirstSQLFlowStmt(`to train a select b from c;`)
	a.Nil(pr)
	a.Equal(-1, idx)
	a.True(strings.Contains(e.Error(), `near or before "select b f`))
}

func TestRemoveCommentInSQLStatement(t *testing.T) {
	testFunc := func(inputSQL, expectedOutputSQL string, noError bool) {
		actualOutputSQL, err := RemoveCommentInSQLStatement(inputSQL)
		if noError {
			assert.NoError(t, err)
			assert.Equal(t, expectedOutputSQL, actualOutputSQL)
			return
		}
		assert.Error(t, err)
	}

	testFunc(`SELECT * FROM a;`, `SELECT * FROM a;`, true)
	testFunc(`SELECT * FROM a -- I am a comment`, `SELECT * FROM a `, true)
	testFunc("SELECT * FROM a -- I am a comment\n TO TRAIN b WITH -- 2nd comment\n INTO c",
		"SELECT * FROM a \n TO TRAIN b WITH \n INTO c", true)
	testFunc("SELECT '--' FROM a -- I am a comment\n TO TRAIN b WITH -- 2nd comment\n INTO c",
		"SELECT '--' FROM a \n TO TRAIN b WITH \n INTO c", true)
	testFunc("SELECT \"abc -- FROM a -- I am a comment\n TO TRAIN b WITH -- 2nd comment\n\" INTO c",
		"SELECT \"abc -- FROM a -- I am a comment\n TO TRAIN b WITH -- 2nd comment\n\" INTO c", true)
	testFunc("SELECT \"abc\\\" -- comment\" FROM b", "SELECT \"abc\\\" -- comment\" FROM b", true)
	testFunc("SELECT \"abc", "", false)
	testFunc("SELECT 'a\\'bc", "", false)

	testFunc("SELECT a /*bc*/ FROM b", "SELECT a   FROM b", true)
	testFunc("SELECT a /**/ FROM b", "SELECT a   FROM b", true)
	testFunc("SELECT a /*/ FROM b", "SELECT a   FROM b", false)
	testFunc("SELECT \"a /*bc*/\" FROM b", "SELECT \"a /*bc*/\" FROM b", true)
	testFunc("/*--hehe*/SELECT \"a /*bc*/\" FROM b", " SELECT \"a /*bc*/\" FROM b", true)
	testFunc("--\n/*--hehe*/SELECT \"a /*bc*/\" FROM b", "\n SELECT \"a /*bc*/\" FROM b", true)
	testFunc("--/*--hehe*/SELECT \"a /*bc*/\" FROM b", "", true)
}

func TestParseSQLStatementWithComment(t *testing.T) {
	sqlWithoutTailingComment := `SELECT * FROM iris.train
TO TRAIN DNNClassifier  -- comment 1
WITH -- comment 2
	model.hidden_units=[16, 32], -- comment 3
	model.n_classes=3 /* comment
4 */
LABEL class --
INTO my_dnn_classifier;`

	tailingComment := `
-- comment 5
-- comment 6
-- /* comment 7 */
`

	for _, dialect := range []string{"mysql", "hive", "maxcompute"} {
		testSQL := sqlWithoutTailingComment
		testTailingComment := tailingComment
		// NOTE(sneaxiy): OdpsParserAdaptor cannot parse /*...*/
		if dialect == "maxcompute" {
			var err error
			testSQL, err = removeMultipleLineComment(testSQL)
			assert.NoError(t, err)
			testTailingComment, err = removeMultipleLineComment(testTailingComment)
			assert.NoError(t, err)
		}

		parsed, err := Parse(dialect, testSQL+testTailingComment)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(parsed))
		assert.Equal(t, testSQL, parsed[0].Original)
		assert.Equal(t, "SELECT * FROM iris.train\n", parsed[0].StandardSelect.String())
		assert.Equal(t, true, parsed[0].IsUnfinishedSelect)
		assert.Equal(t, true, parsed[0].IsExtendedSyntax())
		assert.Equal(t, true, parsed[0].Train)
		assert.Equal(t, "DNNClassifier", parsed[0].Estimator)
		assert.Equal(t, "my_dnn_classifier", parsed[0].Save)
		assert.Equal(t, "class", parsed[0].Label)
		assert.Equal(t, 2, len(parsed[0].TrainAttrs))
		_, ok := parsed[0].TrainAttrs["model.hidden_units"]
		assert.Equal(t, true, ok)
		_, ok = parsed[0].TrainAttrs["model.n_classes"]
		assert.Equal(t, true, ok)
	}
}
