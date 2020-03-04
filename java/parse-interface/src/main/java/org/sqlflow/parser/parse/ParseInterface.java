package org.sqlflow.parser.parse;

public interface ParseInterface {
  // parse calls the underlining parser to parse a SQL program and returns a ParseResult.
  //
  // It returns <statements, -1, ""> if Calcite parser accepts the SQL program.
  //     input:  "select 1; select 1;"
  //     output: {"select 1;", "select 1;"}, -1 , nil
  // It returns <statements, idx, ""> if Calcite parser accepts part of the SQL program,
  // indicated by idx.
  //     input:  "select 1; select 1 to train; select 1"
  //     output: {"select 1;", "select 1"}, 19, nil
  // It returns <nil, -1, error> if an error is occurred.
  ParseResult parse(String sql);

  // dialect returns the SQL dialect of the parser, e.g. "hive", "calcite"
  String dialect();
}
