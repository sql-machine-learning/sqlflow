import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.StatusRuntimeException;
import java.util.concurrent.TimeUnit;
import java.util.logging.Level;
import java.util.logging.Logger;


public class CalciteParserClient {
  private static final Logger logger = Logger.getLogger(CalciteParserClient.class.getName());

  private final ManagedChannel channel;
  private final CalciteParserGrpc.CalciteParserBlockingStub blockingStub;

  public CalciteParserClient(String host, int port) {
    this(ManagedChannelBuilder.forAddress(host, port)
	 .usePlaintext() // No TLS.
        .build());
  }

  CalciteParserClient(ManagedChannel channel) {
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
      rpl = blockingStub.parse(CalciteParserProto.CalciteParserRequest.newBuilder().setQuery(query).build());
    } catch (StatusRuntimeException e) {
      logger.log(Level.WARNING, "RPC failed: {0}", e.getStatus());
      return;
    }
    logger.info("Parse result: " + rpl.getSql() + ", " + rpl.getExtension() + ", " + rpl.getError());
  }

  public static void main(String[] args) throws Exception {
    CalciteParserClient client = new CalciteParserClient("localhost", 50051);
    try {
	String q = "SELECT pn FROM p WHERE pId IN (SELECT pId FROM orders WHERE Quantity > 100)";
      if (args.length > 0) {
        q = args[0];
      }
      client.parse(q);
    } finally {
      client.shutdown();
    }
  }
}
