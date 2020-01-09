package org.sqlflow.parser;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.StatusRuntimeException;
import java.util.concurrent.TimeUnit;
import java.util.logging.Level;
import java.util.logging.Logger;

public class ParserGrpcClient {
  private static final Logger logger = Logger.getLogger(ParserGrpcClient.class.getName());

  private final ManagedChannel channel;
  private final ParserGrpc.ParserBlockingStub blockingStub;

  /** Construct client connecting to HelloWorld server at {@code host:port}. */
  public ParserGrpcClient(String host, int port) {
    this(
        ManagedChannelBuilder.forAddress(host, port)
            // Channels are secure by default (via SSL/TLS). For the example we disable TLS to avoid
            // needing certificates.
            .usePlaintext()
            .build());
  }

  /** Construct client for accessing HelloWorld server using the existing channel. */
  private ParserGrpcClient(ManagedChannel channel) {
    this.channel = channel;
    blockingStub = ParserGrpc.newBlockingStub(channel);
  }

  public void shutdown() throws InterruptedException {
    channel.shutdown().awaitTermination(5, TimeUnit.SECONDS);
  }

  /** Say hello to server. */
  public ParserProto.ParserResponse parse(String sql_program) {
    logger.info("Will try to greet " + sql_program + " ...");
    ParserProto.ParserRequest request =
        ParserProto.ParserRequest.newBuilder().setSqlProgram(sql_program).build();
    ParserProto.ParserResponse response;
    try {
      response = blockingStub.parse(request);
      return response;
    } catch (StatusRuntimeException e) {
      logger.log(Level.WARNING, "RPC failed: {0}", e.getStatus());
      return null;
    }
  }
}
