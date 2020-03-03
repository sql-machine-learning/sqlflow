package org.sqlflow.parser;

import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.StreamObserver;
import java.io.File;
import java.io.IOException;
import java.net.URL;
import java.net.URLClassLoader;
import java.util.ArrayList;
import java.util.concurrent.TimeUnit;
import java.util.logging.Logger;
import org.apache.commons.cli.CommandLine;
import org.apache.commons.cli.CommandLineParser;
import org.apache.commons.cli.DefaultParser;
import org.apache.commons.cli.HelpFormatter;
import org.apache.commons.cli.Options;
import org.apache.commons.cli.ParseException;
import org.sqlflow.parser.ParserProto.ParserRequest;
import org.sqlflow.parser.ParserProto.ParserResponse;
import org.sqlflow.parser.parse.ParseInterface;
import org.sqlflow.parser.parse.ParseResult;

public class ParserGrpcServer {
  private static final Logger logger = Logger.getLogger(ParserGrpcServer.class.getName());

  private final int port;
  private Server server;

  public ParserGrpcServer(int port) {
    this.port = port;
  }

  /** Start serving requests. */
  public void start() throws IOException {
    server = ServerBuilder.forPort(port).addService(new ParserImpl()).build().start();

    logger.info("Server started, listening on " + port);
    Runtime.getRuntime()
        .addShutdownHook(
            new Thread() {
              @Override
              public void run() {
                // Use stderr here since the logger may have been reset by its JVM shutdown hook.
                System.err.println("*** shutting down gRPC server since JVM is shutting down");
                try {
                  ParserGrpcServer.this.stop();
                } catch (InterruptedException e) {
                  e.printStackTrace(System.err);
                }
                System.err.println("*** server shut down");
              }
            });
  }

  private void stop() throws InterruptedException {
    if (server != null) {
      server.shutdown().awaitTermination(30, TimeUnit.SECONDS);
    }
  }

  /** Await termination on the main thread since the grpc library uses daemon threads. */
  private void blockUntilShutdown() throws InterruptedException {
    if (server != null) {
      server.awaitTermination();
    }
  }

  static class ParserImpl extends ParserGrpc.ParserImplBase {
    @Override
    public void parse(ParserRequest request, StreamObserver<ParserResponse> responseObserver) {
      if (!(request.getDialect().equals("calcite") || request.getDialect().equals("hive"))) {
        ParserResponse response =
            ParserResponse.newBuilder()
                .setError(String.format("unrecognized dialect %s", request.getDialect()))
                .build();
        responseObserver.onNext(response);
        responseObserver.onCompleted();
        return;
      }

      String filePath;
      String classPath;
      // TODO(tony): load this from command line argument
      if (request.getDialect().equals("calcite")) {
        filePath = "/opt/sqlflow/parser/parser-calcite-0.0.1-dev-jar-with-dependencies.jar";
        classPath = "org.sqlflow.parser.calcite.CalciteParserAdaptor";
      } else {
        filePath = "/opt/sqlflow/parser/parser-hive-0.0.1-dev-jar-with-dependencies.jar";
        classPath = "org.sqlflow.parser.hive.HiveParserAdaptor";
      }

      ParseResult parseResult = new ParseResult();
      try {
        File file = new File(filePath);
        URL url = file.toURI().toURL();
        URL[] urls = new URL[] {url};
        ClassLoader cl = new URLClassLoader(urls);
        Object parser = cl.loadClass(classPath).newInstance();
        parseResult = ((ParseInterface) parser).parse(request.getSqlProgram());
      } catch (Exception e) {
        parseResult.statements = new ArrayList<String>();
        parseResult.position = -1;
        parseResult.error = e.getClass().getName() + " " + e.getMessage();
      }

      ParserResponse.Builder responseBuilder = ParserResponse.newBuilder();
      responseBuilder.addAllSqlStatements(parseResult.statements);
      responseBuilder.setIndex(parseResult.position);
      responseBuilder.setError(parseResult.error);

      responseObserver.onNext(responseBuilder.build());
      responseObserver.onCompleted();
    }
  }

  /** Main starts a Java Server. */
  public static void main(String[] args) {
    Options options = new Options();
    options.addRequiredOption("p", "port", true, "port number");

    CommandLine line = null;
    try {
      CommandLineParser parser = new DefaultParser();
      line = parser.parse(options, args);
    } catch (ParseException e) {
      HelpFormatter formatter = new HelpFormatter();
      formatter.printHelp("Parser Command Line", options);
      System.exit(1);
    }

    try {
      ParserGrpcServer server = new ParserGrpcServer(Integer.parseInt(line.getOptionValue("p")));
      server.start();
      server.blockUntilShutdown();
    } catch (Exception e) {
      System.err.println("start server failed");
      System.exit(1);
    }
  }
}
