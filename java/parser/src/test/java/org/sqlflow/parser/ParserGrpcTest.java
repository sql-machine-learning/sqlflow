package org.sqlflow.parser;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.fail;

import java.io.IOException;
import org.junit.Before;
import org.junit.Test;

public class ParserGrpcTest {
  private int port = 12345;

  @Before
  public void before() {
    ParserGrpcServer server = new ParserGrpcServer(port);
    try {
      server.start();
    } catch (IOException e) {
      fail("start server failed");
    }
  }

  @Test
  public void testParse() {
    ParserGrpcClient client = new ParserGrpcClient("localhost", port);

    {
      ParserProto.ParserResponse response = client.parse("calcite", "select 1");
      assertEquals(-1, response.getIndex());
      assertEquals(1, response.getSqlStatementsCount());
      assertEquals("select 1", response.getSqlStatements(0));
      assertEquals("", response.getError());
    }

    {
      ParserProto.ParserResponse response = client.parse("hive", "select a from b");
      assertEquals(-1, response.getIndex());
      assertEquals(1, response.getSqlStatementsCount());
      assertEquals("select a from b", response.getSqlStatements(0));
      assertEquals("", response.getError());
    }

    {
      ParserProto.ParserResponse response = client.parse("some_other_dialect", "select a from b");
      assertEquals(0, response.getIndex());
      assertEquals(0, response.getSqlStatementsCount());
      assertEquals("unrecognized dialect some_other_dialect", response.getError());
    }
  }
}
