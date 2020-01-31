package org.sqlflow.parser;

import static org.junit.Assert.*;

import java.io.IOException;
import org.junit.Assert;
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
      Assert.assertEquals(-1, response.getIndex());
      Assert.assertEquals(1, response.getSqlStatementsCount());
      Assert.assertEquals("select 1", response.getSqlStatements(0));
      Assert.assertEquals("", response.getError());
    }

    {
      ParserProto.ParserResponse response = client.parse("hiveql", "select a from b");
      Assert.assertEquals(-1, response.getIndex());
      Assert.assertEquals(1, response.getSqlStatementsCount());
      Assert.assertEquals("select a from b", response.getSqlStatements(0));
      Assert.assertEquals("", response.getError());
    }

    {
      ParserProto.ParserResponse response = client.parse("some_other_dialect", "select a from b");
      Assert.assertEquals(0, response.getIndex());
      Assert.assertEquals(0, response.getSqlStatementsCount());
      Assert.assertEquals("unrecognized dialect some_other_dialect", response.getError());
    }
  }
}
