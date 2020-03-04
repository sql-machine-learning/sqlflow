package org.sqlflow.parser;

import static org.junit.Assert.assertNotNull;

import org.junit.Test;

public class ParserFactoryTest {
  @Test
  public void testDynamicLoading() throws Exception {
    String folderPath = System.getenv("SQLFLOW_PARSER_SERVER_LOADING_PATH");
    assertNotNull(folderPath);
    ParserFactory parserFactory = new ParserFactory(folderPath);
    parserFactory.newParser("hive");
    parserFactory.newParser("calcite");
  }
}
