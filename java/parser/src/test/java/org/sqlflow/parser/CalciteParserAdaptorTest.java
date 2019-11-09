package org.sqlflow.parser;

import static org.junit.Assert.*;

import java.io.File;
import java.io.IOException;
import java.util.ArrayList;
import org.apache.commons.io.FileUtils;
import org.junit.Test;

public class CalciteParserAdaptorTest {
  @Test
  public void testMain() {
    try {
      // FIXME(tony): create file in a temporary directory
      FileUtils.writeByteArrayToFile(new File("test.sql"), "select 1".getBytes());
    } catch (IOException e) {
      fail("create SQL input file failed");
    }

    CalciteParserAdaptor.main(new String[] {"-i", "test.sql", "-o", "output"});

    String output = null;
    try {
      output = new String(FileUtils.readFileToByteArray(new File("output")));
    } catch (IOException e) {
      fail("read parsed output file failed");
    }

    ParseResult parsed_result = new ParseResult();
    parsed_result.Statements = new ArrayList<String>();
    parsed_result.Statements.add("select 1");
    parsed_result.Position = -1;
    parsed_result.Error = "";
    assertEquals(parsed_result.toJSONString(), output);
  }

  @Test
  public void testParseAndSplit() {
    ArrayList<String> standard_select = new ArrayList<String>();
    standard_select.add("select 1");
    standard_select.add("select * from my_table");
    standard_select.add(
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

    // one standard SQL statement
    for (String sql : standard_select) {
      String sql_program = String.format("%s;", sql);
      ParseResult parse_result = (new CalciteParserAdaptor()).ParseAndSplit(sql_program);
      assertEquals(-1, parse_result.Position);
      assertEquals("", parse_result.Error);
      assertEquals(1, parse_result.Statements.size());
      assertEquals(sql, parse_result.Statements.get(0));
    }

    // two standard SQL statements
    for (String sql : standard_select) {
      String sql_program = String.format("%s;%s;", sql, sql);
      ParseResult parse_result = (new CalciteParserAdaptor()).ParseAndSplit(sql_program);
      assertEquals(-1, parse_result.Position);
      assertEquals("", parse_result.Error);
      assertEquals(2, parse_result.Statements.size());
      assertEquals(sql, parse_result.Statements.get(0));
      assertEquals(sql, parse_result.Statements.get(1));
    }

    // two SQL statements, the first one is extendedSQL
    for (String sql : standard_select) {
      String sql_program = String.format("%s to train;%s;", sql, sql);
      ParseResult parse_result = (new CalciteParserAdaptor()).ParseAndSplit(sql_program);
      assertEquals(sql.length() + 1, parse_result.Position);
      assertEquals("", parse_result.Error);
      assertEquals(1, parse_result.Statements.size());
      assertEquals(sql + " ", parse_result.Statements.get(0));
    }

    // two SQL statements, the second one is extendedSQL
    for (String sql : standard_select) {
      String sql_program = String.format("%s;%s to train;", sql, sql);
      ParseResult parse_result = (new CalciteParserAdaptor()).ParseAndSplit(sql_program);
      assertEquals(sql.length() + 1 + sql.length() + 1, parse_result.Position);
      assertEquals("", parse_result.Error);
      assertEquals(2, parse_result.Statements.size());
      assertEquals(sql, parse_result.Statements.get(0));
      assertEquals(sql + " ", parse_result.Statements.get(1));
    }

    { // two SQL statements, the first standard SQL has an error.
      String sql_program = "select select 1; select 1 to train;";
      ParseResult parse_result = (new CalciteParserAdaptor()).ParseAndSplit(sql_program);
      assertEquals(0, parse_result.Statements.size());
      assertEquals(-1, parse_result.Position);
      assertTrue(parse_result.Error.startsWith("Encountered \"select\" at line 1, column 8."));
    }

    // two SQL statements, the second standard SQL has an error.
    for (String sql : standard_select) {
      String sql_program = String.format("%s;select select 1;", sql);
      ParseResult parse_result = (new CalciteParserAdaptor()).ParseAndSplit(sql_program);
      assertEquals(0, parse_result.Statements.size());
      assertEquals(-1, parse_result.Position);
      assertTrue(parse_result.Error.startsWith("Encountered \"select\" at line 1, column 8."));
    }

    { // non select statement before to train
      String sql_program = "describe table to train;";
      ParseResult parse_result = (new CalciteParserAdaptor()).ParseAndSplit(sql_program);
      assertEquals(0, parse_result.Statements.size());
      assertEquals(-1, parse_result.Position);
      assertTrue(parse_result.Error.startsWith("Encountered \"to\" at line 1, column 16."));
    }
  }
}
