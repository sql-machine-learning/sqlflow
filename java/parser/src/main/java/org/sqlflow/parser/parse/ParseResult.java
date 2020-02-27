package org.sqlflow.parser.parse;

import java.util.ArrayList;

// ParseResult contains the parsing result of ParseAndSplit
public class ParseResult {
  // SQL statements accepted by the parser
  public ArrayList<String> Statements;
  // Position where parser raise error while the parser is able
  // to parse the statement before the position.
  public int Position;
  // Errors encountered during parsing.
  public String Error;
}
