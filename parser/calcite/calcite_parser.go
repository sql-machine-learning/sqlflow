//go:generate protoc CalciateParser.proto --go_out=plugins=grpc:.
//
// This package is a gRPC client that implements CalciteParser.proto.
// The server implementation is in CalciteParserServer.java.
package calcite

import (
	context "context"
	"flag"
	"log"
	"time"

	grpc "google.golang.org/grpc"
)

var (
	addr = flag.String("calciate_parser_addr", ":50051", "Listening address of the Calciate parser gRPC server")
	conn *grpc.ClientConn
)

func Parse(sql string) (idx int, err error) {
	conn, err = grpc.Dial(*addr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := NewCalciteParserClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.Parse(ctx, &CalciteParserRequest{Query: sql})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("Greeting: %s", r.Message)
}
