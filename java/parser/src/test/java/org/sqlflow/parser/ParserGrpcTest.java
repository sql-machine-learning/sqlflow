package org.sqlflow.parser;

import static org.junit.Assert.*;

import java.io.IOException;
import org.junit.Assert;
import org.junit.Test;

public class ParserGrpcTest {

  @Test
  public void testDummy() {
    int port = 12345;
    ParserGrpcServer server = new ParserGrpcServer(port);
    try {
      server.start();
    } catch (IOException e) {
      fail("start server failed");
    }
    ParserGrpcClient client = new ParserGrpcClient("localhost", port);

    ParserProto.ParserResponse response = client.parse("this is a sql program");
    Assert.assertEquals(1, response.getIndex());
  }
}
