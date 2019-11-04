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

import java.io.IOException;
import java.lang.reflect.Field;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.Map;
import org.antlr.runtime.RecognitionException;
import org.apache.hadoop.hive.ql.parse.ASTNode;
import org.apache.hadoop.hive.ql.parse.ParseDriver;
import org.apache.hadoop.hive.ql.parse.ParseError;
import org.apache.hadoop.hive.ql.parse.ParseException;
import java.lang.RuntimeException;

import static org.sqlflow.parser.util.ParserUtils.*;

public class HiveQLParser {
  /**
   * CalciteParser returns { "epos" : -1, "error" : ""} if Calcite parser accepts the query
   * Returns { "epos" : ***, "error" : ""} if a second parsing accepts the content to the left
   * of the error position from the first parsing, otherwise,
   * Returns { "epos" : -1 "error" : "***"} if both parsing failed.
   */ 
  public static void main( String[] args ) {
    if (args.length != 2) {
      throw new IllegalArgumentException("HiveQLParser needs only two arguments. 1. output path, 2. HiveQL");
    }
  	try {
  		writeToFile(args[0], parse(args[1]));
  	} catch (Throwable e) {
      e.printStackTrace();
  	}
  }

  public static String parse(String sql) throws IOException {
    int epos = -1; // Don't use query.length(), use -1.
    String err = "";

    try {
      ParseDriver pd = new ParseDriver();
      ASTNode node = pd.parse(sql);

    } catch (ParseException e) {
      try {
        Field errorsField = ParseException.class.getDeclaredField("errors");
        errorsField.setAccessible(true);
        ArrayList<ParseError> errors = (ArrayList<ParseError>) errorsField.get(e);
        Field reField = ParseError.class.getDeclaredField("re");
        reField.setAccessible(true);
        RecognitionException re = (RecognitionException) reField.get(errors.get(0));
        epos = posToIndex(sql, re.line, re.charPositionInLine);
      } catch (Exception all) {
        err = "Cannot parse the error message from HiveQL parser";
      }

      if (err == "") {
        try {
          ParseDriver pd = new ParseDriver();
          ASTNode node = pd.parse(sql);
        } catch (ParseException ee) {
          err = ee.getMessage();
        } catch (RuntimeException eee) {
          err = eee.getMessage();
        }
      }
    }

    Map<String, Object> result = new HashMap<>();
    result.put("epos", epos);
    result.put("error", err);
    return serializeToString(result);
  }
}
