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
    logger.info("Will try to parse " + query + " ...");
    CalciteParserProto.CalciteParserReply rpl;
    try {
      rpl =
          blockingStub.parse(
              CalciteParserProto.CalciteParserRequest.newBuilder().setQuery(query).build());
    } catch (StatusRuntimeException e) {
      logger.log(Level.WARNING, "RPC failed: {0}", e.getStatus());
      return;
    }
    logger.info(
        "Parse result: " + rpl.getSql() + ", " + rpl.getExtension() + ", " + rpl.getError());
  }

  public static void main(String[] args) throws Exception {
    CalciteParserTest t = new CalciteParserTest("localhost", 50051);
    try {
      String q = "SELECT pn FROM p WHERE pId IN (SELECT pId FROM orders WHERE Quantity > 100)";
      if (args.length > 0) {
        q = args[0];
      }
      t.parse(q);
    } finally {
      t.shutdown();
    }
  }
}
