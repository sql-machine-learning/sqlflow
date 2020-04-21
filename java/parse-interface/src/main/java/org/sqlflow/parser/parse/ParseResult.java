package org.sqlflow.parser.parse;

import java.util.List;

// ParseResult contains the parsing result of parse
public class ParseResult {
  // SQL statements accepted by the parser
  public List<String> statements;
  // position where parser raise error while the parser is able
  // to parse the statement before the position.
  public int position;
  // Errors encountered during parsing.
  public String error;
  // tables that each statement manipulates.
  public List<InputOutputTables> inputOutputTables;
  // Is the SELECT statement unfinished, e.g. at SELECT ... TO [TRAIN|PREDICT|EXPLAIN]
  //                                                       ^
  public boolean isUnfinishedSelect;
}
