package org.sqlflow.parser;

import static org.junit.Assert.*;

import java.io.File;
import java.io.IOException;
import java.util.ArrayList;
import org.apache.commons.io.FileUtils;
import org.junit.Test;

public class ParserAdaptorTest {
  @Test
  public void testCmd() {
    try {
      // FIXME(tony): create file in a temporary directory
      FileUtils.writeByteArrayToFile(new File("test.sql"), "select 1".getBytes());
    } catch (IOException e) {
      fail("create SQL input file failed");
    }

    String[] parser_types = new String[] {"calcite", "hiveql"};
    for (String parser_type : parser_types) {
      ParserAdaptorCmd.main(new String[] {"-p", parser_type, "-i", "test.sql", "-o", "output"});

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
  }
}
