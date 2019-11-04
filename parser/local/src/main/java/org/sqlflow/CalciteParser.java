// Copyright 2019 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package org.sqlflow.parser;

import org.apache.calcite.sql.SqlNode;
import org.apache.calcite.sql.parser.SqlParseException;
import org.apache.calcite.sql.parser.SqlParser;
import org.apache.calcite.sql.parser.SqlParserPos;
import java.util.HashMap;
import java.util.Map;
import java.io.IOException;
import java.lang.IllegalArgumentException;

import static org.sqlflow.parser.util.ParserUtils.*;

public class CalciteParser {
  /**
   * CalciteParser returns { "epos" : -1, "error" : ""} if Calcite parser accepts the query
   * Returns { "epos" : ***, "error" : ""} if a second parsing accepts the content to the left
   * of the error position from the first parsing, otherwise,
   * Returns { "epos" : -1 "error" : "***"} if both parsing failed.
   */ 
  public static void main( String[] args ) {
    if (args.length != 1) {
      throw new IllegalArgumentException("CalciteParser needs two arguments. 1. output path, 2. Calcite SQL");
    }
    try {
      writeToFile(args[0], parse(args[1]));
  	} catch (IOException e) {
      e.printStackTrace();
  	}
  }

  public static String parse(String sql) throws IOException {
    int epos = -1; // Don't use query.length(), use -1.
    String err = "";

    try {
      SqlParser parser = SqlParser.create(sql);
      SqlNode sqlNode = parser.parseQuery();

    } catch (SqlParseException e) {
      SqlParserPos pos = e.getPos();
      epos = posToIndex(sql, pos.getLineNum(), pos.getColumnNum());

      try {
        SqlParser parser = SqlParser.create(sql.substring(0, epos));
        SqlNode sqlNode = parser.parseQuery();
      } catch (SqlParseException ee) {
        err = ee.getCause().getMessage();
      }
    }

    Map<String, Object> result = new HashMap<>();
    result.put("epos", epos);
    result.put("error", err);
    return serializeToString(result);
  }
}
