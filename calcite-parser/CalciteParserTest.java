import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.StatusRuntimeException;
import java.util.concurrent.TimeUnit;
import java.util.logging.Level;
import java.util.logging.Logger;

public class CalciteParserTest {
  private static final Logger logger = Logger.getLogger(CalciteParserTest.class.getName());

  private final ManagedChannel channel;
  private final CalciteParserGrpc.CalciteParserBlockingStub blockingStub;

  public CalciteParserTest(String host, int port) {
    this(
        ManagedChannelBuilder.forAddress(host, port)
            .usePlaintext() // No TLS.
            .build());
  }

  CalciteParserTest(ManagedChannel channel) {
    this.channel = channel;
    blockingStub = CalciteParserGrpc.newBlockingStub(channel);
  }

  public void shutdown() throws InterruptedException {
    channel.shutdown().awaitTermination(5, TimeUnit.SECONDS);
  }

  public void parse(String query) {
    CalciteParserProto.CalciteParserReply rpl;
    try {
      rpl =
          blockingStub.parse(
              CalciteParserProto.CalciteParserRequest.newBuilder().setQuery(query).build());
      if (rpl.getError() != "") {
        logger.log(Level.WARNING, "Unexpected parsing error: {0}", rpl.getError());
        System.exit(-1);
      }
    } catch (StatusRuntimeException e) {
      logger.log(Level.WARNING, "RPC failed: {0}", e.getStatus());
      System.exit(-1);
    }
  }

  public static void main(String[] args) throws Exception {
    CalciteParserTest t = new CalciteParserTest("localhost", 50051);
    try {
      if (args.length > 0) {
        t.parse(args[0]);
      } else {
        System.err.println("Need a command line argment of a SQL statement");
        return;
      }
    } finally {
      t.shutdown();
    }
  }
}
