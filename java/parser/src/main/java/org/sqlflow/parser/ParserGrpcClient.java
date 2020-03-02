package org.sqlflow.parser;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import java.util.concurrent.TimeUnit;
import java.util.logging.Logger;

public class ParserGrpcClient {
  private static final Logger logger = Logger.getLogger(ParserGrpcClient.class.getName());

  private final ManagedChannel channel;
  private final ParserGrpc.ParserBlockingStub blockingStub;

  /**
   * ParserGrpcClient
   *
   * <p>Constructs client connecting to Parser server at {@code host:port}.
   */
  public ParserGrpcClient(String host, int port) {
    this(
        ManagedChannelBuilder.forAddress(host, port)
            // Channels are secure by default (via SSL/TLS). For the example we disable TLS to avoid
            // needing certificates.
            .usePlaintext()
            .build());
  }

  /** Construct client for accessing Parser server using the existing channel. */
  private ParserGrpcClient(ManagedChannel channel) {
    this.channel = channel;
    blockingStub = ParserGrpc.newBlockingStub(channel);
  }

  public void shutdown() throws InterruptedException {
    channel.shutdown().awaitTermination(5, TimeUnit.SECONDS);
  }

  /**
   * parse
   *
   * <p>Implements client side gRPC parse.
   */
  public ParserProto.ParserResponse parse(String dialect, String sqlProgram) {
    logger.info("Will try to greet " + sqlProgram + " ...");
    ParserProto.ParserRequest request =
        ParserProto.ParserRequest.newBuilder()
            .setDialect(dialect)
            .setSqlProgram(sqlProgram)
            .build();
    return blockingStub.parse(request);
  }
}
