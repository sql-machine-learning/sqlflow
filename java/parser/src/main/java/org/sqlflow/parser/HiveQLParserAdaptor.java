package org.sqlflow.parser;

import java.lang.reflect.Field;
import java.util.ArrayList;
import org.antlr.runtime.RecognitionException;
import org.apache.hadoop.hive.ql.Context;
import org.apache.hadoop.hive.ql.parse.ParseDriver;
import org.apache.hadoop.hive.ql.parse.ParseError;
import org.apache.hadoop.hive.ql.parse.ParseException;

public class HiveQLParserAdaptor {

  public HiveQLParserAdaptor() {}

  private ParseResult parseResultError(String error) {
    ParseResult parse_result = new ParseResult();
    parse_result.Statements = new ArrayList<String>();
    parse_result.Position = -1;
    parse_result.Error = error;

    return parse_result;
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
        ParseDriver pd = new ParseDriver();
        pd.parse(sql); // Possibly throw ParseException

        parse_result.Statements.add(sql);
        return parse_result;
      } catch (ParseException e) {
        // Find error position
        int epos = -1;
        try {
          Field errorsField = ParseException.class.getDeclaredField("errors");
          errorsField.setAccessible(true);
          ArrayList<ParseError> errors = (ArrayList<ParseError>) errorsField.get(e);
          Field reField = ParseError.class.getDeclaredField("re");
          reField.setAccessible(true);
          RecognitionException re = (RecognitionException) reField.get(errors.get(0));

          // Note(tony): Calcite parser raise error at the first letter of the error word,
          // while HiveQL parser raise error on the position right before the error word.
          // Consider select 1 to train, Calcite parser raise error at letter t of "to",
          // while HiveQL parser raise error at the white space before "to". As a result,
          // we put `+ 1` on the `epos`.
          epos = posToIndex(sql, re.line, re.charPositionInLine) + 1;
        } catch (Exception all) {
          return parseResultError("Cannot parse the error message from HiveQL parser");
        }

        try {
          ParseDriver pd = new ParseDriver();
          String sub_sql = sql.substring(0, epos);

          pd.parse(sub_sql); // Possibly throws ParseException

          parse_result.Statements.add(sub_sql);

          // multiple SQL statements
          if (sql.charAt(epos) == ';') {
            sql = sql.substring(epos + 1);
            accumulated_position += epos + 1;

            // FIXME(tony): trim is not enough to handle statements
            // like "select 1; select 1; -- this is a comment".
            // So maybe we need some preprocessors to remove all the comments first.
            if (sql.trim().equals("")) {
              return parse_result;
            }

            continue;
          }

          // Make sure the left hand side is a select statement, so that
          // we can try parse the right hand side with the SQLFlow parser
          try {
            // If it is not a select statement, ParseException will be thrown
            pd.parseSelect(sub_sql, (Context) null);
          } catch (ParseException ee) {
            // return original error
            return parseResultError(e.getMessage());
          }
          parse_result.Position = accumulated_position + epos;
          return parse_result;
        } catch (ParseException ee) {
          // return original error
          return parseResultError(e.getMessage());
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
