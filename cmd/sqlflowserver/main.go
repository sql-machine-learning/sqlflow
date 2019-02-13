// To run this program:
//	go generate .. && go run main.go
//
package main

import (
	"database/sql"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/go-sql-driver/mysql"
	"gitlab.alipay-inc.com/Arc/sqlflow/server"
	pb "gitlab.alipay-inc.com/Arc/sqlflow/server/proto"
	sqlflow "gitlab.alipay-inc.com/Arc/sqlflow/sql"
)

const (
	port = ":50051"
)

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
		return
	}
	myCfg := &mysql.Config{
		User:   "root",
		Passwd: "root",
		Addr:   "localhost:3306",
	}
	db, err := sql.Open("mysql", myCfg.FormatDSN())
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
		return
	}
	defer db.Close()

	s := grpc.NewServer()
	pb.RegisterSQLFlowServer(s, server.NewServer(sqlflow.Run, db))
	// Register reflection service on gRPC server.
	reflection.Register(s)
	log.Println("Server Started at", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
