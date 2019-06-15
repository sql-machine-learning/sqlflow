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
package org.sqlflow.parser.remote.calcite;

import io.grpc.stub.StreamObserver;
import java.io.IOException;
import org.apache.calcite.sql.SqlNode;
import org.apache.calcite.sql.parser.SqlParseException;
import org.apache.calcite.sql.parser.SqlParser;
import org.sqlflow.parser.remote.ParserGrpc;
import org.sqlflow.parser.remote.ParserProto;
import org.sqlflow.parser.remote.ParserServer;

public class CalciteParserServer extends ParserServer {

  public static void main(String[] args) throws IOException, InterruptedException {
    s.start(new CalciteParserServer(), parsePort(args, 50051));
    s.blockUntilShutdown();
  }

  static class CalciteParserImpl extends ParserGrpc.ParserImplBase {

    @Override
    public void parse(
        ParserProto.ParserRequest request,
        StreamObserver<ParserProto.ParserReply> responseObserver) {

      int epos = -1; // Don't use query.length(), use -1.
      String err = "";

      try {
        SqlParser parser = SqlParser.create(query);
        SqlNode sqlNode = parser.parseQuery();

      } catch (SqlParseException e) {
        SqlParsePos pos = e.getPos();
        epos = ParserServer.posToIndex(query, pos.getLineNum(), pos.getColumnNum());

        try {
          SqlParser parser = SqlParser.create(query.substring(0, epos));
          SqlNode sqlNode = parser.parseQuery();

        } catch (SqlParseException ee) {
          err = ee.getCause().getMessage();
        }
        err = "";
      }

      responseObserver.onNext(
          CalciteParserProto.CalciteParserReply.newBuilder().setIndex(pos).setError(err).build());
      responseObserver.onCompleted();
    }
  }
}
