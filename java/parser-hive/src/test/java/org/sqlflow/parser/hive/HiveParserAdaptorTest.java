package org.sqlflow.parser.hive;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertTrue;

import java.util.ArrayList;
import org.junit.Test;
import org.sqlflow.parser.parse.ParseResult;

public class HiveParserAdaptorTest {
  @Test
  public void testHiveParseAndSplit() {
    HiveParserAdaptor parser = new HiveParserAdaptor();
    ArrayList<String> standardSelect = new ArrayList<String>();
    standardSelect.add("select 1");
    standardSelect.add("select * from my_table");
    standardSelect.add("select * from\n" + "my_table");
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
      ParseResult parseResult = parser.parse(sqlProgram);
      assertEquals(25, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(1, parseResult.statements.size());
      assertEquals("SELECT *\n" + "FROM iris.train\n", parseResult.statements.get(0));
    }

    // one standard SQL statement
    for (String sql : standardSelect) {
      String sqlProgram = String.format("%s;", sql);
      ParseResult parseResult = parser.parse(sqlProgram);
      assertEquals(-1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(1, parseResult.statements.size());
      assertEquals(sql, parseResult.statements.get(0));
    }

    // two standard SQL statements
    for (String sql : standardSelect) {
      String sqlProgram = String.format("%s;%s;", sql, sql);
      ParseResult parseResult = parser.parse(sqlProgram);
      assertEquals(-1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(2, parseResult.statements.size());
      assertEquals(sql, parseResult.statements.get(0));
      assertEquals(sql, parseResult.statements.get(1));
    }

    // two SQL statements, the first one is extendedSQL
    for (String sql : standardSelect) {
      String sqlProgram = String.format("%s to train;%s;", sql, sql);
      ParseResult parseResult = parser.parse(sqlProgram);
      assertEquals(sql.length() + 1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(1, parseResult.statements.size());
      assertEquals(sql + " ", parseResult.statements.get(0));
    }

    // two SQL statements, the second one is extendedSQL
    for (String sql : standardSelect) {
      String sqlProgram = String.format("%s;%s to train;", sql, sql);
      ParseResult parseResult = parser.parse(sqlProgram);
      assertEquals(sql.length() + 1 + sql.length() + 1, parseResult.position);
      assertEquals("", parseResult.error);
      assertEquals(2, parseResult.statements.size());
      assertEquals(sql, parseResult.statements.get(0));
      assertEquals(sql + " ", parseResult.statements.get(1));
    }

    { // two SQL statements, the first standard SQL has an error.
      String sqlProgram = "select select 1; select 1 to train;";
      ParseResult parseResult = parser.parse(sqlProgram);
      assertEquals(0, parseResult.statements.size());
      assertEquals(-1, parseResult.position);
      assertTrue(
          parseResult.error.startsWith(
              "line 1:7 cannot recognize input near 'select' '1' ';' in select clause"));
    }

    // two SQL statements, the second standard SQL has an error.
    for (String sql : standardSelect) {
      String sqlProgram = String.format("%s;select select 1;", sql);
      ParseResult parseResult = parser.parse(sqlProgram);
      assertEquals(0, parseResult.statements.size());
      assertEquals(-1, parseResult.position);
      assertTrue(
          parseResult.error.startsWith(
              "line 1:7 cannot recognize input near 'select' '1' ';' in select clause"));
    }

    { // non select statement before to train
      String sqlProgram = "describe my_table to train;";
      ParseResult parseResult = parser.parse(sqlProgram);
      assertEquals(0, parseResult.statements.size());
      assertEquals(-1, parseResult.position);
      assertTrue(parseResult.error.startsWith("line 1:18 missing EOF at 'to' near 'my_table"));
    }
  }
}
