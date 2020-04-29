package org.sqlflow.parser.parse;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

public abstract class BaseParser implements ParseInterface {

  /**
   * parse one statement and return where error occurs.
   *
   * @param statement contains only one sql statement
   * @param stmtResult parse result of this statement, input/ouput tables will be merged in final
   *     result
   * @return -1 on successfully parse the statement, otherwise return where error occurs
   * @throws Exception means impossible to parse
   */
  protected abstract int parseOneStmt(String statement, ParseResult stmtResult) throws Exception;

  /**
   * is given statement a selection statement.
   *
   * @param statement only contains on statement
   * @return true if it is a select statement
   */
  protected abstract boolean isSelectStmt(String statement);

  /**
   * split sql into statements, do not trim the result, we will calculate length base on the result.
   *
   * @param sql is a program with multiple statements
   * @return pieces each is a statement
   */
  protected abstract List<String> splitStatements(String sql);

  /**
   * get leading comment's length.
   *
   * @param sql is a program which may contain comments at its head
   * @return leading comment length
   */
  protected abstract int getLeadingCommentLen(String sql);

  @Override
  public ParseResult parse(String sql) {
    ParseResult parseResult = new ParseResult();
    parseResult.statements = new ArrayList<String>();
    parseResult.inputOutputTables = new ArrayList<>();
    parseResult.position = 0;
    parseResult.error = "";
    parseResult.isUnfinishedSelect = false;

    try {
      boolean noErr = true;
      List<String> stmts = this.splitStatements(sql);
      for (int i = 0; i < stmts.size(); ++i) {
        String stmt = stmts.get(i);
        ParseResult stmtResult = new ParseResult();
        int pos = parseOneStmt(stmt, stmtResult);
        if (pos == 0) { // error at the first word
          noErr = false;
          break;
        } else if (pos < 0) { // accepted
          parseResult.statements.add(stmt);
          parseResult.position += stmt.length();
          mergeResult(parseResult, stmtResult);
        } else if (pos == stmt.length() - 1 && stmt.charAt(pos) == ';') {
          // some parser do not accept the last ';'
          parseResult.statements.add(stmt.substring(0, pos));
          parseResult.position += stmt.length();
          mergeResult(parseResult, stmtResult);
        } else { // error at pos
          noErr = false;
          String prefix = stmt.substring(0, pos);
          stmtResult = new ParseResult();
          if (parseOneStmt(prefix, stmtResult) < 0) {
            if (isSelectStmt(prefix)) {
              parseResult.isUnfinishedSelect = true;
              parseResult.statements.add(prefix);
              parseResult.position += prefix.length();
              mergeResult(parseResult, stmtResult);
            }
            // else return current parsed stmts
          }
          break;
        }
      }
      // accept the whole sql
      if (noErr) {
        parseResult.position = -1;
      }
    } catch (Exception | Error e) {
      // exception means we can't parse the sql
      parseResult.statements = Collections.emptyList();
      parseResult.position = -1;
      parseResult.error = e.getMessage();
    }
    // we are in middle of sql, trim comments because
    // our extended parser do not accept comments
    if (parseResult.position >= 0) {
      String unparsed = sql.substring(parseResult.position);
      parseResult.position += getLeadingCommentLen(unparsed);
    }
    return parseResult;
  }

  private void mergeResult(ParseResult parseResult, ParseResult stmtResult) {
    if (stmtResult.inputOutputTables != null) {
      parseResult.inputOutputTables.addAll(stmtResult.inputOutputTables);
    }
  }

  protected static int posToIndex(String query, int line, int column) {
    int l = 0;
    int c = 0;

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
