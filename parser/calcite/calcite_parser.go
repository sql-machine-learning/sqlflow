//go:generate protoc CalciteParser.proto --go_out=plugins=grpc:.
//
// This package is a gRPC client that implements CalciteParser.proto.
// The server implementation is in CalciteParserServer.java.
package calcite

import (
	"context"
	"fmt"
	"time"

	grpc "google.golang.org/grpc"
)

var (
	// Use global variables to avoid reconnecting for every call.
	conn   *grpc.ClientConn
	client CalciteParserClient
)

// Init the connection to the Calcite parser gRPC server.
func Init(addr string) error {
	Cleanup()
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("Cannot connect to Calcite parser gRPC server: %v", err)
	}
	client = NewCalciteParserClient(conn)
	return nil
}

// Cleanup the connection to the gRPC server.
func Cleanup() {
	if conn != nil {
		conn.Close()
	}
}

// Parse a SQL statement by calling the gRPC server.  Parse doesn't
// require sql ends with ';'.  It returns (-1, nil) if Calcite parser
// accepts sql, (idx,nil) if Calcite parser accepts sql[0:idx], or,
// (idx,err) if Calcite parse cannot parse sql as a whole or as a
// part.  The index idx is the position where the first parsing fails.
// We are supposed to call SQLFlow parser with sql[idx:] if Parse
// returns (idx,nil).
func Parse(sql string) (idx int, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := client.Parse(ctx, &CalciteParserRequest{Query: sql})
	if err != nil {
		return -1, fmt.Errorf("gRPC call error: %v", err)
	}
	if r.GetError() != "" {
		return int(r.GetIndex()), fmt.Errorf(r.GetError())
	}
	return int(r.GetIndex()), nil
}
