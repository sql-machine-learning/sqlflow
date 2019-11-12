package org.sqlflow.parser;

import java.util.ArrayList;
import org.json.simple.JSONArray;
import org.json.simple.JSONObject;

// ParseResult contains the parsing result of ParseAndSplit
class ParseResult {
  // SQL statements accepted by the parser
  ArrayList<String> Statements;
  // Position where parser raise error while the parser is able
  // to parse the statement before the position.
  int Position;
  // Errors encountered during parsing.
  String Error;

  String toJSONString() {
    JSONObject obj = new JSONObject();

    JSONArray list = new JSONArray();
    for (String s : Statements) {
      list.add(s);
    }
    obj.put("statements", list);
    obj.put("position", Position);
    obj.put("error", Error);

    return obj.toJSONString();
  }
}
