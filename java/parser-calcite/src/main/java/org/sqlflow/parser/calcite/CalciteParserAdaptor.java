package org.sqlflow.parser.calcite;

import java.io.StringReader;
import java.util.LinkedList;
import java.util.List;
import org.apache.calcite.sql.SqlKind;
import org.apache.calcite.sql.SqlNode;
import org.apache.calcite.sql.parser.SqlParseException;
import org.apache.calcite.sql.parser.SqlParser;
import org.apache.calcite.sql.parser.ddl.SimpleCharStream;
import org.apache.calcite.sql.parser.ddl.SqlDdlParserImpl;
import org.apache.calcite.sql.parser.ddl.SqlDdlParserImplTokenManager;
import org.apache.calcite.sql.parser.ddl.Token;
import org.sqlflow.parser.parse.BaseParser;
import org.sqlflow.parser.parse.ParseResult;

public class CalciteParserAdaptor extends BaseParser {

  public CalciteParserAdaptor() {}

  @Override
  public String dialect() {
    return "calcite";
  }

  @Override
  protected int parseOneStmt(String sql, ParseResult result) throws Exception {
    try {
      SqlParser.Config sqlParserConfig =
          SqlParser.configBuilder().setParserFactory(SqlDdlParserImpl.FACTORY).build();
      SqlParser parser = SqlParser.create(sql, sqlParserConfig);
      parser.parseQuery();
      return -1;
    } catch (SqlParseException e) {
      return posToIndex(sql, e.getPos().getLineNum(), e.getPos().getColumnNum());
    }
  }

  @Override
  protected boolean isSelectStmt(String sql) {
    SqlParser.Config sqlParserConfig =
        SqlParser.configBuilder().setParserFactory(SqlDdlParserImpl.FACTORY).build();
    SqlParser parser = SqlParser.create(sql, sqlParserConfig);
    try {
      SqlNode node = parser.parseQuery();
      return SqlKind.QUERY.contains(node.getKind());
    } catch (SqlParseException e) {
      return false;
    }
  }

  @Override
  protected List<String> splitStatements(String sql) {
    List<String> stmts = new LinkedList<>();
    SimpleCharStream stream =
        new SimpleCharStream(new StringReader(sql)) {
          {
            super.setTabSize(1);
          }
        };
    // this lexer will automatically filter comments
    SqlDdlParserImplTokenManager tm = new SqlDdlParserImplTokenManager(stream);
    int pos = 0;
    boolean hasToken = false;
    while (true) {
      Token token = tm.getNextToken();
      if (token.kind == SqlDdlParserImplTokenManager.EOF) {
        break;
      }
      hasToken = true;
      if (token.kind == SqlDdlParserImplTokenManager.SEMICOLON) {
        int e = 1 + posToIndex(sql, token.beginLine, token.beginColumn);
        stmts.add(sql.substring(pos, e));
        pos = e;
        hasToken = false;
      }
    }
    // if the last part has no token, we discard it
    if (pos < sql.length() && hasToken) {
      // TODO(lhw) find a better way than modify original SQL statements
      // At this point, if the last char == ';', it must be in a comment,
      // in case the parser report an error at this commented ';'
      // we replace this ';' to ' ' (still keep it's length).
      // It's a problem when parser stop at a commented ';' because we think ';'
      // as separator of statements, so we may falsely accept this query.
      // Fortunately, there is only one case it will happen, that is the last
      // char in a query is ';' AND is commented AND is where error reported (
      // actually, the parser is stopping at EOF, but the error will be reported
      // at ';')
      // Example: 'SELECT -- comment ;'
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
    SimpleCharStream stream =
        new SimpleCharStream(new StringReader(sql)) {
          {
            super.setTabSize(1);
          }
        };
    // this lexer will automatically filter comments
    SqlDdlParserImplTokenManager tm = new SqlDdlParserImplTokenManager(stream);
    Token token = tm.getNextToken();
    if (token.kind == SqlDdlParserImplTokenManager.EOF) {
      return sql.length();
    }
    return posToIndex(sql, token.beginLine, token.beginColumn);
  }
}
