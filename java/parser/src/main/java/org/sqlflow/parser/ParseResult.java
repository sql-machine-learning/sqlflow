package org.sqlflow.parser;

import java.util.ArrayList;

// ParseResult contains the parsing result of ParseAndSplit
class ParseResult {
  // SQL statements accepted by the parser
  ArrayList<String> Statements;
  // Position where parser raise error while the parser is able
  // to parse the statement before the position.
  int Position;
  // Errors encountered during parsing.
  String Error;
}
