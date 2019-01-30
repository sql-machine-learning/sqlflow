// To run this program:
//	go generate .. && go run main.go
//
package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/wangkuiyi/sqlflow/server"
)

const (
	port = ":50051"
)

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	server.RegisterSQLFlowServer(s, &server.Server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)
	log.Println("Server Started at", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
