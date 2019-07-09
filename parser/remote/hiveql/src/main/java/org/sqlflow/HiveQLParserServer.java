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
package org.sqlflow.parser.remote.hiveql;

import io.grpc.stub.StreamObserver;
import java.io.IOException;
import java.lang.reflect.Field;
import java.util.ArrayList;
import org.antlr.runtime.RecognitionException;
import org.apache.hadoop.hive.ql.parse.ParseDriver;
import org.apache.hadoop.hive.ql.parse.ParseError;
import org.apache.hadoop.hive.ql.parse.ParseException;
import org.sqlflow.parser.remote.ParserGrpc;
import org.sqlflow.parser.remote.ParserProto;
import org.sqlflow.parser.remote.ParserServer;

public class HiveQLParserServer extends ParserServer {

  public static void main(String[] args) throws IOException, InterruptedException {
    HiveQLParserServer s = new HiveQLParserServer();
    s.start(new HiveQLParserImpl(), parsePort(args, 50052));
    s.blockUntilShutdown();
  }

  static class HiveQLParserImpl extends ParserGrpc.ParserImplBase {

    // parse returns <-1,null> if HiveQL parser accepts the query, or
    // <pos,null> if a second parsing accepts the content to the left
    // of the error position from the first parsing, otherwise,
    // <-1,err> if both parsing failed.
    @Override
    public void parse(
        ParserProto.ParserRequest request,
        StreamObserver<ParserProto.ParserResponse> responseObserver) {

      String q = request.getQuery();
      int epos = -1; // Don't use query.length(), use -1.
      String err = "";

      try {
        ParseDriver pd = new ParseDriver();
        pd.parse(q);

      } catch (ParseException e) {
        try {
          Field errorsField = ParseException.class.getDeclaredField("errors");
          errorsField.setAccessible(true);
          ArrayList<ParseError> errors = (ArrayList<ParseError>) errorsField.get(e);
          Field reField = ParseError.class.getDeclaredField("re");
          reField.setAccessible(true);
          RecognitionException re = (RecognitionException) reField.get(errors.get(0));
          epos = ParserServer.posToIndex(q, re.line, re.charPositionInLine);
        } catch (Exception all) {
          err = "Cannot parse the error message from HiveQL parser";
        }

        if (Strings.isNullOrEmpty(err)) {
          try {
            ParseDriver pd = new ParseDriver();
            pd.parse(q);
          } catch (ParseException ee) {
            err = ee.getMessage();
          }
        }
      }

      responseObserver.onNext(
          ParserProto.ParserResponse.newBuilder().setIndex(epos).setError(err).build());
      responseObserver.onCompleted();
    }
  }
}
