//go:generate protoc CalciateParser.proto --go_out=plugins=grpc:.
//
// This package is a gRPC client that implements CalciteParser.proto.
// The server implementation is in CalciteParserServer.java.
package calcite

func Parse(sql string) (err error, idx int) {

}
