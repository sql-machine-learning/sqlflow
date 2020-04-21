package org.sqlflow.parser.calcite;

import static org.junit.Assert.assertEquals;

import java.util.ArrayList;
import java.util.List;
import org.junit.Test;
import org.sqlflow.parser.parse.ParseResult;

public class CalciteParserAdaptorTest {
  @Test
  public void testParseAndSplit() {
    ArrayList<String> standardSelect = new ArrayList<String>();
    standardSelect.add("select 1");
    standardSelect.add("select * from my_table");
    standardSelect.add(
        "SELECT\n"
            + "customerNumber,  \n"
            + "    customerName \n"
            + "FROM \n"
            + "    customers \n"
            + "WHERE \n"
            + "    EXISTS( SELECT  \n"
            + "            orderNumber, SUM(priceEach * quantityOrdered) \n"
            + "        FROM \n"
            + "            orderdetails \n"
            + "                INNER JOIN \n"
            + "            orders USING (orderNumber) \n"
            + "        WHERE \n"
            + "            customerNumber = customers.customerNumber \n"
            + "        GROUP BY orderNumber \n"
            + "        HAVING SUM(priceEach * quantityOrdered) > 60000)");

    {
      String sql = "create table ttt as (select * from iris.train)";
      String sqlProgram = String.format("%s;", sql);
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(-1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(1, parseResult.statements.size());
      assertEquals(sql, parseResult.statements.get(0));
    }

    {
      String sqlProgram =
          "SELECT *\n"
              + "FROM iris.train\n"
              + "TO TRAIN xgboost.gbtree\n"
              + "WITH\n"
              + "    objective=\"multi:softprob\",\n"
              + "    train.num_boost_round = 30,\n"
              + "    eta = 0.4,\n"
              + "    num_class = 3\n"
              + "COLUMN sepal_length, sepal_width, petal_length, petal_width\n"
              + "LABEL class \n"
              + "INTO sqlflow_models.my_xgboost_model;";
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(25, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(1, parseResult.statements.size());
      assertEquals("SELECT *\n" + "FROM iris.train\n", parseResult.statements.get(0));
    }

    // empty stmt
    {
      String sqlProgram = "";
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(-1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(0, parseResult.statements.size());
    }

    // one empty stmt
    {
      String sql = "-- ; \n/**\n**/";
      String sqlProgram = String.format("%s;", sql);
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(-1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(1, parseResult.statements.size());
      assertEquals(sql, parseResult.statements.get(0));
    }

    // one standard SQL statement
    for (String sql : standardSelect) {
      String sqlProgram = String.format("%s;", sql);
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(-1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(1, parseResult.statements.size());
      assertEquals(sql, parseResult.statements.get(0));
    }

    // two standard SQL statements
    for (String sql : standardSelect) {
      String sqlProgram = String.format("%s;%s;", sql, sql);
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(-1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(2, parseResult.statements.size());
      assertEquals(sql, parseResult.statements.get(0));
      assertEquals(sql, parseResult.statements.get(1));
    }

    // two SQL statements, the first one is extendedSQL
    for (String sql : standardSelect) {
      String sqlProgram = String.format("%s to train;%s;", sql, sql);
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(sql.length() + 1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(1, parseResult.statements.size());
      assertEquals(sql + " ", parseResult.statements.get(0));
      assertEquals(true, parseResult.isUnfinishedSelect);
    }

    // two SQL statements, the second one is extendedSQL
    for (String sql : standardSelect) {
      String sqlProgram = String.format("%s;%s to train;", sql, sql);
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(sql.length() + 1 + sql.length() + 1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(2, parseResult.statements.size());
      assertEquals(sql, parseResult.statements.get(0));
      assertEquals(sql + " ", parseResult.statements.get(1));
      assertEquals(true, parseResult.isUnfinishedSelect);
    }

    { // two SQL statements, the first standard SQL has an error.
      String sqlProgram = "select select 1; select 1 to train;";
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(0, parseResult.statements.size());
      assertEquals(0, parseResult.position);
      assertEquals("", parseResult.error);
      sqlProgram = "show train my_model; select 1;";
      parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(0, parseResult.statements.size());
      assertEquals(0, parseResult.position);
      assertEquals("", parseResult.error);
    }

    // two SQL statements, the second standard SQL has an error.
    for (String sql : standardSelect) {
      String sqlProgram = String.format("%s;select select 1;", sql);
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(1, parseResult.statements.size());
      assertEquals(sql.length() + 1, parseResult.position);
      assertEquals("", parseResult.error);
    }

    // one union statement
    for (String sql : standardSelect) {
      String union = String.format("%s union %s", sql, sql);
      String sqlProgram = String.format("%s to train my_model", union);
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals("", parseResult.error);
      assertEquals(true, parseResult.isUnfinishedSelect);
      assertEquals(1, parseResult.statements.size());
      assertEquals(union.length() + 1, parseResult.position);
    }

    // non select statement before to train
    {
      String sqlProgram = "describe table to train;";
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(0, parseResult.statements.size());
      assertEquals(0, parseResult.position);
      assertEquals("", parseResult.error);
    }

    // show train statement
    {
      String sqlProgram = "show train my_model";
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(0, parseResult.statements.size());
      assertEquals(0, parseResult.position);
      assertEquals("", parseResult.error);
    }
    {
      String sqlProgram = "select 1; show train my_model;";
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(1, parseResult.statements.size());
      assertEquals(10, parseResult.position);
      assertEquals("", parseResult.error);
    }
    {
      String sqlProgram = "select 1 TO TRAIN; show train my_model; select 1;";
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(1, parseResult.statements.size());
      assertEquals(9, parseResult.position);
      assertEquals("", parseResult.error);
    }

    // can't parse because of incomplete comment
    {
      String sqlProgram = "/* foget comment back select 1;";
      ParseResult parseResult = (new CalciteParserAdaptor()).parse(sqlProgram);
      assertEquals(0, parseResult.statements.size());
      assertEquals(-1, parseResult.position);
      assertEquals(
          "Lexical error at line 1, column 32.  " + "Encountered: <EOF> after : \"\"",
          parseResult.error);
    }
  }

  @Test
  public void testSplitStmt() {
    CalciteParserAdaptor parser = new CalciteParserAdaptor();
    // no valid stmt
    {
      String sqlProgram = "--";
      List<String> stmts = parser.splitStatements(sqlProgram);
      assertEquals(0, stmts.size());

      sqlProgram = "--; \n/*\n;\n*/ \n--;-- ;";
      stmts = parser.splitStatements(sqlProgram);
      assertEquals(0, stmts.size());
    }
    // empty stmt
    {
      String sqlProgram = ";";
      List<String> stmts = parser.splitStatements(sqlProgram);
      assertEquals(1, stmts.size());
      assertEquals(";", stmts.get(0));

      sqlProgram = ";;";
      stmts = parser.splitStatements(sqlProgram);
      assertEquals(2, stmts.size());

      sqlProgram = "; ;  ;   ";
      stmts = parser.splitStatements(sqlProgram);
      assertEquals(3, stmts.size());
      assertEquals(";", stmts.get(0));
      assertEquals(" ;", stmts.get(1));
      assertEquals("  ;", stmts.get(2));
    }
    // one stmt
    {
      String sqlProgram = "select 1";
      List<String> stmts = parser.splitStatements(sqlProgram);
      assertEquals(1, stmts.size());
      assertEquals(sqlProgram, stmts.get(0));
    }
    {
      String sqlProgram = "select 1;";
      List<String> stmts = parser.splitStatements(sqlProgram);
      assertEquals(1, stmts.size());
      assertEquals(sqlProgram, stmts.get(0));
    }

    // two stmt
    {
      String single = "select 1";
      String sqlProgram = String.format("%s;%s", single, single);
      List<String> stmts = parser.splitStatements(sqlProgram);
      assertEquals(2, stmts.size());
      assertEquals(single + ";", stmts.get(0));
      assertEquals(single, stmts.get(1));
    }
    {
      String single = "select 1;";
      String sqlProgram = String.format("%s%s", single, single);
      List<String> stmts = parser.splitStatements(sqlProgram);
      assertEquals(2, stmts.size());
      assertEquals(single, stmts.get(0));
      assertEquals(single, stmts.get(1));
    }

    // end with comment(contains ';')
    {
      String single = "select 1";
      String sqlProgram = String.format("%s;%s --;", single, single);
      List<String> stmts = parser.splitStatements(sqlProgram);
      assertEquals(2, stmts.size());
      assertEquals(single + ";", stmts.get(0));
      assertEquals(single + " -- ", stmts.get(1));
    }

    // discard only-comment tail
    {
      String single = "select 1;";
      String sqlProgram = String.format("%s --;", single);
      List<String> stmts = parser.splitStatements(sqlProgram);
      assertEquals(1, stmts.size());
      assertEquals(single, stmts.get(0));
    }

    // comment in middle
    {
      String single = "select 1;";
      String sqlProgram = String.format("%s--comment\n%s", single, single);
      List<String> stmts = parser.splitStatements(sqlProgram);
      assertEquals(2, stmts.size());
      assertEquals(single, stmts.get(0));
      assertEquals("--comment\n" + single, stmts.get(1));
    }
  }

  @Test
  public void testGetLeadingCommentLen() {
    CalciteParserAdaptor p = new CalciteParserAdaptor();
    assertEquals(0, p.getLeadingCommentLen(null));
    assertEquals(0, p.getLeadingCommentLen(""));
    // tab width 1
    assertEquals(1, p.getLeadingCommentLen("\tTO train"));
    assertEquals(0, p.getLeadingCommentLen("TO train"));
    assertEquals(12, p.getLeadingCommentLen("-- comment \nTO train"));
    assertEquals(12, p.getLeadingCommentLen("--\n--a\n--abc"));
    assertEquals(13, p.getLeadingCommentLen("--\n--a\n--abc\nSELECT"));
    assertEquals(12, p.getLeadingCommentLen("-- comment \nSELECT\n--comment"));
    assertEquals(13, p.getLeadingCommentLen("/* commnt */ hello"));
    assertEquals(27, p.getLeadingCommentLen("/* commnt */\n/* comment */ he"));
    assertEquals(13, p.getLeadingCommentLen("/* commnt */ hello /* cot */ he"));
    assertEquals(25, p.getLeadingCommentLen("--comment \n/* comment;*/ SHOW"));
    assertEquals(25, p.getLeadingCommentLen("/* comment;*/--comment \n SHOW"));
    assertEquals(25, p.getLeadingCommentLen("/* comment;*/\n--comment\n SHOW"));
  }
}
