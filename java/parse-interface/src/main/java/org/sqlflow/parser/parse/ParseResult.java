package org.sqlflow.parser.parse;

import java.util.ArrayList;

// ParseResult contains the parsing result of parse
public class ParseResult {
  // SQL statements accepted by the parser
  public ArrayList<String> statements;
  // position where parser raise error while the parser is able
  // to parse the statement before the position.
  public int position;
  // Errors encountered during parsing.
  public String error;
}
