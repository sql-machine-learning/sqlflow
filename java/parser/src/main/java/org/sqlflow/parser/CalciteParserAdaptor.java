package org.sqlflow.parser;

import java.util.ArrayList;
import org.apache.calcite.sql.SqlKind;
import org.apache.calcite.sql.SqlNode;
import org.apache.calcite.sql.parser.SqlParseException;
import org.apache.calcite.sql.parser.SqlParser;
import org.apache.calcite.sql.parser.ddl.SqlDdlParserImpl;

public class CalciteParserAdaptor {

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
      SqlParser.Config sqlParserConfig =
          SqlParser.configBuilder().setParserFactory(SqlDdlParserImpl.FACTORY).build();
      try {
        SqlParser parser = SqlParser.create(sql, sqlParserConfig);
        SqlNode sqlnode = parser.parseQuery();
        parse_result.Statements.add(sql);
        return parse_result;
      } catch (SqlParseException e) {
        int line = e.getPos().getLineNum();
        int column = e.getPos().getColumnNum();
        int epos = posToIndex(sql, line, column);

        try {
          SqlParser parser = SqlParser.create(sql.substring(0, epos), sqlParserConfig);
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

          // Make sure the left hand side is a query, so that
          // we can try parse the right hand side with the SQLFlow parser
          // SqlKind.QUERY is {SELECT, EXCEPT, INTERSECT, UNION, VALUES, ORDER_BY, EXPLICIT_TABLE}
          if (!SqlKind.QUERY.contains(sqlnode.getKind())) {
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
