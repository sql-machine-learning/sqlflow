package org.sqlflow.parser.hive;

import java.lang.reflect.Field;
import java.util.ArrayList;
import java.util.LinkedList;
import java.util.List;
import org.antlr.runtime.RecognitionException;
import org.antlr.runtime.Token;
import org.apache.hadoop.hive.ql.parse.ParseDriver;
import org.apache.hadoop.hive.ql.parse.ParseDriver.ANTLRNoCaseStringStream;
import org.apache.hadoop.hive.ql.parse.ParseDriver.HiveLexerX;
import org.apache.hadoop.hive.ql.parse.ParseError;
import org.apache.hadoop.hive.ql.parse.ParseException;
import org.sqlflow.parser.parse.BaseParser;
import org.sqlflow.parser.parse.ParseResult;

public class HiveParserAdaptor extends BaseParser {
  private static Field errorsField;
  private static Field reField;
  private ParseDriver parseDriver = new ParseDriver();

  // This static block is to initialize some reflection field
  // which aids us to get error message from parse Exception.
  // Static block only execute only when this class is loaded.
  static {
    try {
      errorsField = ParseException.class.getDeclaredField("errors");
      reField = ParseError.class.getDeclaredField("re");
      errorsField.setAccessible(true);
      reField.setAccessible(true);
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }

  public HiveParserAdaptor() {}

  @Override
  public String dialect() {
    return "hive";
  }

  @SuppressWarnings("unchecked")
  @Override
  protected int parseOneStmt(String sql, ParseResult result) throws Exception {
    try {
      parseDriver.parse(sql); // Possibly throw ParseException
      return -1;
    } catch (ParseException e) {
      // Find error position
      try {
        ArrayList<ParseError> errors;
        RecognitionException re;
        errors = (ArrayList<ParseError>) errorsField.get(e);
        re = (RecognitionException) reField.get(errors.get(0));

        // Note(tony): Calcite parser raise error at the first letter of the
        // error word, while Hive parser raise error on the position right
        // before the error word.
        // Consider select 1 to train, Calcite parser raise error at letter t of
        // "to", while Hive parser raise error at the white space before "to".
        // As a result, we put `+ 1` on the `epos`.
        return posToIndex(sql, re.line, re.charPositionInLine + 1);
      } catch (Exception all) {
        throw new Exception("Cannot parse the error message from Hive parser");
      }
    }
  }

  @Override
  protected boolean isSelectStmt(String sql) {
    try {
      parseDriver.parseSelect(sql, null);
      return true;
    } catch (ParseException e) {
      return false;
    }
  }

  @Override
  protected List<String> splitStatements(String sql) {
    List<String> stmts = new LinkedList<>();
    ANTLRNoCaseStringStream stream = parseDriver.new ANTLRNoCaseStringStream(sql);
    HiveLexerX lexer = parseDriver.new HiveLexerX(stream);
    int pos = 0;
    boolean hasToken = false;
    while (true) {
      Token token = lexer.nextToken();
      if (token.getType() == HiveLexerX.EOF) {
        break;
      }
      if (token.getChannel() == Token.HIDDEN_CHANNEL) {
        continue;
      }
      hasToken = true;
      if (token.getType() == HiveLexerX.SEMICOLON) {
        int end = posToIndex(sql, token.getLine(), token.getCharPositionInLine() + 1) + 1;
        stmts.add(sql.substring(pos, end));
        pos = end;
        hasToken = false;
      }
    }
    if (pos < sql.length() && hasToken) {
      String lastStmt = sql.substring(pos);
      lastStmt = lastStmt.replaceAll(";$", " ");
      stmts.add(lastStmt);
    }
    return stmts;
  }

  @Override
  protected int getLeadingCommentLen(String sql) {
    if (sql == null) {
      return 0;
    }
    ANTLRNoCaseStringStream stream = parseDriver.new ANTLRNoCaseStringStream(sql);
    HiveLexerX lexer = parseDriver.new HiveLexerX(stream);
    Token token = null;
    while (true) {
      token = lexer.nextToken();
      if (token.getChannel() != Token.HIDDEN_CHANNEL) {
        break;
      }
    }
    if (token == null) {
      return 0;
    }
    return posToIndex(sql, token.getLine(), token.getCharPositionInLine() + 1);
  }
}
