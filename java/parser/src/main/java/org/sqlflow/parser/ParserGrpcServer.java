package org.sqlflow.parser;

import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.StreamObserver;
import java.io.IOException;
import java.util.ArrayList;
import java.util.concurrent.TimeUnit;
import org.apache.commons.cli.CommandLine;
import org.apache.commons.cli.CommandLineParser;
import org.apache.commons.cli.DefaultParser;
import org.apache.commons.cli.HelpFormatter;
import org.apache.commons.cli.Options;
import org.apache.commons.cli.ParseException;
import org.sqlflow.parser.ParserProto.InputOutputTables;
import org.sqlflow.parser.ParserProto.ParserRequest;
import org.sqlflow.parser.ParserProto.ParserResponse;
import org.sqlflow.parser.parse.ParseInterface;
import org.sqlflow.parser.parse.ParseResult;

public class ParserGrpcServer {
  private ParserFactory parserFactory;
  private final int port;
  private Server server;

  public ParserGrpcServer(int port, String loadPath) throws Exception {
    this.port = port;
    this.parserFactory = new ParserFactory(loadPath);
  }

  /** Start serving requests. */
  public void start() throws IOException {
    server = ServerBuilder.forPort(port).addService(new ParserImpl()).build().start();

    System.err.println("Server started, listening on " + port);
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

  class ParserImpl extends ParserGrpc.ParserImplBase {
    @Override
    public void parse(ParserRequest request, StreamObserver<ParserResponse> responseObserver) {
      ParseResult parseResult = new ParseResult();
      try {
        ParseInterface parser = parserFactory.newParser(request.getDialect());
        parseResult = parser.parse(request.getSqlProgram());
      } catch (Exception e) {
        parseResult.statements = new ArrayList<String>();
        parseResult.position = -1;
        parseResult.error = e.getClass().getName() + " " + e.getMessage();
        parseResult.isUnfinishedSelect = false;
      }

      ParserResponse.Builder responseBuilder = ParserResponse.newBuilder();
      responseBuilder.addAllSqlStatements(parseResult.statements);
      responseBuilder.setIndex(parseResult.position);
      responseBuilder.setError(parseResult.error);
      responseBuilder.setIsUnfinishedSelect(parseResult.isUnfinishedSelect);
      if (parseResult.inputOutputTables != null) {
        for (int i = 0; i < parseResult.inputOutputTables.size(); i++) {
          InputOutputTables.Builder tablesBuilder = InputOutputTables.newBuilder();
          tablesBuilder.addAllInputs(parseResult.inputOutputTables.get(i).inputTables);
          tablesBuilder.addAllOutputs(parseResult.inputOutputTables.get(i).outputTables);
          responseBuilder.addInputOutputTables(tablesBuilder);
        }
      }

      responseObserver.onNext(responseBuilder.build());
      responseObserver.onCompleted();
    }
  }

  /** Main starts a Java Server. */
  public static void main(String[] args) {
    Options options = new Options();
    options.addRequiredOption("p", "port", true, "port number");
    options.addRequiredOption("l", "loadPath", true, ".jar files path for dynamic loading");
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
      ParserGrpcServer server =
          new ParserGrpcServer(
              Integer.parseInt(line.getOptionValue("p")), line.getOptionValue("l"));
      server.start();
      server.blockUntilShutdown();
    } catch (Exception e) {
      System.err.println("start server failed");
      System.err.printf("%s: %s\n", e.getClass(), e.getMessage());
      System.exit(1);
    }
  }
}
