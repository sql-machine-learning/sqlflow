package org.sqlflow.parser.parse;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

public abstract class BaseParser implements ParseInterface {

  /**
   * parse one stmt from the start of sql.
   *
   * @param sql sql program
   * @return successfully parse all the sql -> return -1; on error -> return where error occurs;
   * @throws Exception means impossilbe to parse
   */
  protected abstract int parseOneStmt(String sql) throws Exception;

  /**
   * is given sql a selection stmt.
   *
   * @param sql sql program
   * @return a valid extend context is set
   */
  protected abstract boolean isSelectionStmt(String sql);

  /**
   * split sql into statements, do not trim the result, we will cal length based on the result.
   *
   * @param sql sql program
   * @return splited pieces
   */
  protected abstract List<String> splitStatements(String sql);

  /**
   * get leading comment's length.
   *
   * @param sql sql program
   * @return leading comment length
   */
  protected abstract int getLeadingCommentLen(String sql);

  @Override
  public ParseResult parse(String sql) {
    ParseResult parseResult = new ParseResult();
    parseResult.statements = new ArrayList<String>();
    parseResult.position = 0;
    parseResult.error = "";
    parseResult.isUnfinishedSelect = false;

    try {
      boolean noErr = true;
      List<String> stmts = this.splitStatements(sql);
      for (int i = 0; i < stmts.size(); ++i) {
        String stmt = stmts.get(i);
        int pos = parseOneStmt(stmt);
        if (pos == 0) { // error at the first word
          noErr = false;
          break;
        } else if (pos < 0) { // accepted
          parseResult.statements.add(stmt);
          parseResult.position += stmt.length();
        } else if (pos == stmt.length() - 1 && stmt.charAt(pos) == ';') {
          // some parse do not accept the last ';'
          parseResult.statements.add(stmt.substring(0, pos));
          parseResult.position += stmt.length();
        } else { // error at pos
          noErr = false;
          String prefix = stmt.substring(0, pos);
          if (parseOneStmt(prefix) < 0) {
            if (isSelectionStmt(prefix)) {
              parseResult.isUnfinishedSelect = true;
              parseResult.statements.add(prefix);
              parseResult.position += prefix.length();
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
    // we are in middle of sql, trim comment because
    // our extended parser do not accept comments
    if (parseResult.position >= 0) {
      parseResult.position += getLeadingCommentLen(sql.substring(parseResult.position));
    }
    return parseResult;
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
