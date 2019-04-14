import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.StreamObserver;
import java.io.IOException;
import java.util.logging.Logger;
import javafx.util.Pair;
import org.apache.calcite.sql.SqlNode;
import org.apache.calcite.sql.parser.SqlParseException;
import org.apache.calcite.sql.parser.SqlParser;
import org.apache.calcite.sql.parser.SqlParserPos;

public class CalciteParserServer {
  private static final Logger logger = Logger.getLogger(CalciteParserServer.class.getName());

  private Server server;

  private void start() throws IOException {
    /* The port on which the server should run */
    int port = 50051;
    server = ServerBuilder.forPort(port).addService(new CalciteParserImpl()).build().start();
    logger.info("Server started, listening on " + port);
    Runtime.getRuntime()
        .addShutdownHook(
            new Thread() {
              @Override
              public void run() {
                System.err.println("*** shutting down gRPC server since JVM is shutting down");
                CalciteParserServer.this.stop();
                System.err.println("*** server shut down");
              }
            });
  }

  private void stop() {
    if (server != null) {
      server.shutdown();
    }
  }

  private void blockUntilShutdown() throws InterruptedException {
    if (server != null) {
      server.awaitTermination();
    }
  }

  public static void main(String[] args) throws IOException, InterruptedException {
    final CalciteParserServer s = new CalciteParserServer();
    s.start();
    s.blockUntilShutdown();
  }

  static class CalciteParserImpl extends CalciteParserGrpc.CalciteParserImplBase {

    @Override
    public void parse(
        CalciteParserProto.CalciteParserRequest request,
        StreamObserver<CalciteParserProto.CalciteParserReply> responseObserver) {

	String q = request.getQuery();
	Pair<Integer, SqlParseException> r = calcite(q);
	if (r.getValue() == null) {
	    int i = r.getKey();
	    responseObserver.onNext(CalciteParserProto.CalciteParserReply.
				    newBuilder().
				    setSql(q.substring(0, i)).
				    setExtension(q.substring(i)).
				    build());
	} else {
	    responseObserver.onNext(CalciteParserProto.CalciteParserReply.
				    newBuilder().
				    setError(r.getValue().getCause().getMessage()).
				    build());
	}
	responseObserver.onCompleted();
    }

    private static int PosToIndex(String query, SqlParserPos pos) {
      int line = 0, column = 0;

      for (int i = 0; i < query.length(); i++) {
        if (line == pos.getLineNum() - 1 && column == pos.getColumnNum() - 1) {
          return i;
        }

        if (query.charAt(i) == '\n') {
          line++;
          column = 0;
        } else {
          column++;
        }
      }

      return query.length();
    }

    private static Pair<Integer, SqlParseException> calcite(String query) {
      try {
        SqlParser parser = SqlParser.create(query);
        SqlNode sqlNode = parser.parseQuery();

      } catch (SqlParseException e) {
        int epos = PosToIndex(query, e.getPos());
        try {
          SqlParser parser = SqlParser.create(query.substring(0, epos));
          SqlNode sqlNode = parser.parseQuery();
        } catch (SqlParseException ee) {
          return new Pair<Integer, SqlParseException>(epos, ee);
        }
        return new Pair<Integer, SqlParseException>(epos, null);
      }

      return new Pair<Integer, SqlParseException>(query.length(), null);
    }
  }
}
