package org.sqlflow.parser;

import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.StreamObserver;
import java.io.IOException;
import java.util.concurrent.TimeUnit;
import java.util.logging.Logger;
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
      // TODO(tony): Implement this function to call actual parsers
      ParserResponse response = ParserResponse.newBuilder().setIndex(1).build();
      responseObserver.onNext(response);
      responseObserver.onCompleted();
    }
  }
}
