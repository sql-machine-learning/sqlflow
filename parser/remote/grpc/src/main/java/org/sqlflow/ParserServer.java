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
package org.sqlflow.parser.remote;

import io.grpc.Server;
import io.grpc.ServerBuilder;
import java.io.IOException;
import org.apache.commons.cli.CommandLine;
import org.apache.commons.cli.CommandLineParser;
import org.apache.commons.cli.DefaultParser;
import org.apache.commons.cli.OptionBuilder;
import org.apache.commons.cli.Options;
import org.apache.commons.cli.ParseException;
import org.apache.log4j.Logger;

/* A class derived from ParserServer is supposed to define method main as follows:

   public static void main(String[] args) throws IOException, InterruptedException {
     s.start(new CalciteParserServer(), parsePort(args, 500021));
     s.blockUntilShutdown();
   }
*/
public class ParserServer {
  protected static Logger logger = Logger.getLogger(ParserServer.class.getName());
  protected Server server;

  protected void start(ParserGrpc.ParserImplBase serverImpl, int port) throws IOException {
    server = ServerBuilder.forPort(port).addService(serverImpl).build().start();
    logger.info("Server started, listening on " + port);
    Runtime.getRuntime()
        .addShutdownHook(
            new Thread() {
              @Override
              public void run() {
                System.err.println("*** shutting down gRPC server since JVM is shutting down");
                ParserServer.this.stop();
                System.err.println("*** server shut down");
              }
            });
  }

  protected void stop() {
    if (server != null) {
      server.shutdown();
    }
  }

  protected void blockUntilShutdown() throws InterruptedException {
    if (server != null) {
      server.awaitTermination();
    }
  }

  protected static int parsePort(String[] args, int defaultPort) {
    try {
      CommandLineParser parser = new DefaultParser();
      Options options = new Options();
      options.addOption(
          OptionBuilder.withArgName("port")
              .hasArg()
              .withDescription("the port to listen on")
              .create("port"));
      CommandLine line = parser.parse(options, args);
      if (line.hasOption("port")) {
        return Integer.parseInt(line.getOptionValue("port"));
      }
    } catch (ParseException e) {
      System.err.printf(
          "Command line options error: %s. Use default port %s\n", e.getMessage(), defaultPort);
    }
    return defaultPort;
  }

  // posToIndex converts line and column number into string index.
  protected static int posToIndex(String query, int line, int column) {
    int l = 0, c = 0;

    for (int i = 0; i < query.length(); i++) {
      if (l == line - 1 && c == column - 1) {
        return i;
      }

      if (query.charAt(i) == '\n') {
        l++;
        c = 0;
      } else {
        c++;
      }
    }

    return query.length();
  }
}
