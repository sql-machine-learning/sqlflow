package org.sqlflow.parser;

import java.util.ArrayList;
import org.apache.calcite.sql.SqlKind;
import org.apache.calcite.sql.SqlNode;
import org.apache.calcite.sql.parser.SqlParseException;
import org.apache.calcite.sql.parser.SqlParser;
import org.apache.log4j.Logger;

public class CalciteParserAdaptor {

  static final Logger logger = Logger.getLogger(CalciteParserAdaptor.class);

  class ParseResult {
    public ArrayList<String> Statements;
    public int Position;
    public String Error;
  };

  public CalciteParserAdaptor() {}

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
        logger.debug("1 ----------------");
        logger.debug(sql);
        parse_result.Statements.add(sql);
        return parse_result;
      } catch (SqlParseException e) {
        int line = e.getPos().getLineNum();
        int column = e.getPos().getColumnNum();
        int pos = posToIndex(sql, line, column);

        try {
          SqlParser parser = SqlParser.create(sql.substring(0, pos));
          SqlNode sqlnode = parser.parseQuery();

          // parseQuery doesn't throw exception
          parse_result.Statements.add(sql.substring(0, pos));

          // multiple SQL statements
          if (sql.charAt(pos) == ';') {
            logger.debug("2.1 -----------------");
            logger.debug(sql.substring(0, pos));

            sql = sql.substring(pos + 1);
            accumulated_position += pos + 1;

            // FIXME(tony): trim is not enough to handle statements
            // like "select 1; select 1; -- this is a comment"
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

          logger.debug("2.2 -----------------");
          logger.debug(sql.substring(0, pos));
          parse_result.Position = accumulated_position + pos;
          return parse_result;
        } catch (SqlParseException ee) {
          logger.debug("3 -----------------");

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
