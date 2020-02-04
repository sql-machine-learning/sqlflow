package org.sqlflow.parser;

import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.StreamObserver;
import java.io.IOException;
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
      if (!(request.getDialect().equals("calcite") || request.getDialect().equals("hiveql"))) {
        ParserResponse response =
            ParserResponse.newBuilder()
                .setError(String.format("unrecognized dialect %s", request.getDialect()))
                .build();
        responseObserver.onNext(response);
        responseObserver.onCompleted();
        return;
      }

      ParseResult parse_result;
      if (request.getDialect().equals("calcite")) {
        CalciteParserAdaptor parser = new CalciteParserAdaptor();
        parse_result = parser.ParseAndSplit(request.getSqlProgram());
      } else {
        HiveQLParserAdaptor parser = new HiveQLParserAdaptor();
        parse_result = parser.ParseAndSplit(request.getSqlProgram());
      }

      ParserResponse.Builder response_builder = ParserResponse.newBuilder();
      response_builder.addAllSqlStatements(parse_result.Statements);
      response_builder.setIndex(parse_result.Position);
      response_builder.setError(parse_result.Error);

      responseObserver.onNext(response_builder.build());
      responseObserver.onCompleted();
    }
  }

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
