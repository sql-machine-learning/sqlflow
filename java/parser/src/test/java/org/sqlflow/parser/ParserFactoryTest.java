package org.sqlflow.parser;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNotNull;

import org.junit.Test;
import org.sqlflow.parser.parse.ParseInterface;

public class ParserFactoryTest {
  @Test
  public void testDynamicLoading() throws Exception {
    String folderPath = System.getenv("SQLFLOW_PARSER_SERVER_LOADING_PATH");
    assertNotNull(folderPath);
    ParserFactory parserFactory = new ParserFactory(folderPath);
    assertEquals("hive", parserFactory.newParser("hive").dialect());
    assertEquals("calcite", parserFactory.newParser("calcite").dialect());
  }

  @Test
  public void testMaxcompute() throws Exception {
    String folderPath = System.getenv("SQLFLOW_PARSER_SERVER_LOADING_PATH");
    assertNotNull(folderPath);
    ParserFactory parserFactory = new ParserFactory(folderPath);
    try {
      ClassLoader cl = Thread.currentThread().getContextClassLoader();
      cl.loadClass("org.sqlflow.parser.OdpsParserAdaptor");
      ParseInterface mcParaser = parserFactory.newParser("maxcompute");
      assertEquals("odps", mcParaser.dialect());
    } catch (ClassNotFoundException e) {
      ParseInterface mcParaser = parserFactory.newParser("maxcompute");
      assertEquals("calcite", mcParaser.dialect());
    }
  }
}
