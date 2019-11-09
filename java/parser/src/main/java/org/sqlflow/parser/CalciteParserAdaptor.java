package org.sqlflow.parser;

import java.io.File;
import java.io.FileWriter;
import java.io.IOException;
import java.util.ArrayList;
import org.apache.calcite.sql.SqlKind;
import org.apache.calcite.sql.SqlNode;
import org.apache.calcite.sql.parser.SqlParseException;
import org.apache.calcite.sql.parser.SqlParser;
import org.apache.commons.cli.CommandLine;
import org.apache.commons.cli.CommandLineParser;
import org.apache.commons.cli.DefaultParser;
import org.apache.commons.cli.HelpFormatter;
import org.apache.commons.cli.Options;
import org.apache.commons.cli.ParseException;
import org.apache.commons.io.FileUtils;

public class CalciteParserAdaptor {

  public CalciteParserAdaptor() {}

  public static void main(String[] args) {
    Options options = new Options();
    options.addRequiredOption("i", "input_file", true, "input SQL file");
    options.addRequiredOption("o", "output_file", true, "output parsed result file");

    CommandLine line = null;
    try {
      CommandLineParser parser = new DefaultParser();
      line = parser.parse(options, args);
    } catch (ParseException e) {
      HelpFormatter formatter = new HelpFormatter();
      formatter.printHelp("Parser Command Line", options);
      System.exit(-1);
    }

    String input_file = line.getOptionValue("i");
    String output_file = line.getOptionValue("o");
    String content = null;
    try {
      content = new String(FileUtils.readFileToByteArray(new File(input_file)));
    } catch (IOException e) {
      e.printStackTrace();
      System.exit(-1);
    }

    CalciteParserAdaptor parser = new CalciteParserAdaptor();
    String jsonString = parser.ParseAndSplit(content).toJSONString();

    try {
      FileWriter file = new FileWriter(output_file);
      file.write(jsonString);
      file.flush();
    } catch (IOException e) {
      e.printStackTrace();
      System.exit(-1);
    }
  }

  // ParseAndSplit calls Calcite parser to parse a SQL program and returns a ParseResult.
  //
  // It returns <statements, -1, ""> if Calcite parser accepts the SQL program.
  //     input:  "select 1; select 1;"
  //     output: {"select 1;", "select 1;"}, -1 , nil
  // It returns <statements, idx, ""> if Calcite parser accepts part of the SQL program, indicated
  // by idx.
  //     input:  "select 1; select 1 to train; select 1"
  //     output: {"select 1;", "select 1"}, 19, nil
  // It returns <nil, -1, error> if an error is occurred.
  public ParseResult ParseAndSplit(String sql) {
    ParseResult parse_result = new ParseResult();
    parse_result.Statements = new ArrayList<String>();
    parse_result.Position = -1;
    parse_result.Error = "";

    int accumulated_position = 0;
    while (true) {
      try {
        SqlParser parser = SqlParser.create(sql);
        SqlNode sqlnode = parser.parseQuery();
        parse_result.Statements.add(sql);
        return parse_result;
      } catch (SqlParseException e) {
        int line = e.getPos().getLineNum();
        int column = e.getPos().getColumnNum();
        int epos = posToIndex(sql, line, column);

        try {
          SqlParser parser = SqlParser.create(sql.substring(0, epos));
          SqlNode sqlnode = parser.parseQuery();

          // parseQuery doesn't throw exception
          parse_result.Statements.add(sql.substring(0, epos));

          // multiple SQL statements
          if (sql.charAt(epos) == ';') {
            sql = sql.substring(epos + 1);
            accumulated_position += epos + 1;

            // FIXME(tony): trim is not enough to handle statements
            // like "select 1; select 1; -- this is a comment"
            // So maybe we need some preprocessors to remove all the comments first.
            if (sql.trim().equals("")) {
              return parse_result;
            }

            continue;
          }

          // Make sure the left hand side is a select statement, so that
          // we can try parse the right hand side with the SQLFlow parser
          if (sqlnode.getKind() != SqlKind.SELECT) {
            // return original error
            parse_result.Statements = new ArrayList<String>();
            parse_result.Position = -1;
            parse_result.Error = e.getCause().getMessage();
            return parse_result;
          }

          parse_result.Position = accumulated_position + epos;
          return parse_result;
        } catch (SqlParseException ee) {
          // return original error
          parse_result.Statements = new ArrayList<String>();
          parse_result.Position = -1;
          parse_result.Error = e.getCause().getMessage();

          return parse_result;
        }
      }
    }
  }

  // posToIndex converts line and column number into string index.
  private static int posToIndex(String query, int line, int column) {
    int l = 0, c = 0;

    for (int i = 0; i < query.length(); i++) {
      if (l == line - 1 && c == column - 1) {
        return i;
      }

      if (query.charAt(i) == '\n') {
        l++;
        c = 0;
      } else {
        c++;
      }
    }

    return query.length();
  }
}
