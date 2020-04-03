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

public class CalciteParserAdaptor extends BaseParser {

  public CalciteParserAdaptor() {}

  @Override
  public String dialect() {
    return "calcite";
  }

  @Override
  protected int parseOneStmt(String sql) throws Exception {
    try {
      SqlParser.Config sqlParserConfig =
          SqlParser.configBuilder().setParserFactory(SqlDdlParserImpl.FACTORY).build();
      SqlParser parser = SqlParser.create(sql, sqlParserConfig);
      parser.parseQuery();
      return -1;
    } catch (SqlParseException e) {
      return posToIndex(
          sql,
          e.getPos().getLineNum(), //
          e.getPos().getColumnNum());
    }
  }

  @Override
  protected boolean isSelectionStmt(String sql) {
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
    // this lexer will auto filter comments
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
    // if last part has no token, we discard it
    if (pos < sql.length() && hasToken) {
      stmts.add(sql.substring(pos));
    }
    return stmts;
  }

  public static void main(String[] args) throws Exception {
    CalciteParserAdaptor p = new CalciteParserAdaptor();
    System.out.println(p.splitStatements("\tBAD;"));
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
    // this lexer will auto filter comments
    SqlDdlParserImplTokenManager tm = new SqlDdlParserImplTokenManager(stream);
    Token token = tm.getNextToken();
    if (token.kind == SqlDdlParserImplTokenManager.EOF) {
      return sql.length();
    }
    return posToIndex(sql, token.beginLine, token.beginColumn);
  }
}
