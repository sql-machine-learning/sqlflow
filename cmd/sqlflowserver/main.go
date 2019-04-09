// To run this program:
//	go generate .. && go run main.go
//
package main

import (
	"flag"
	"log"
	"net"

	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/sql-machine-learning/sqlflow/server"
	pb "github.com/sql-machine-learning/sqlflow/server/proto"
	sf "github.com/sql-machine-learning/sqlflow/sql"
)

const (
	port = ":50051"
)

func main() {
	user := flag.String("db_user", "root", "database user name")
	pswd := flag.String("db_password", "root", "database user password")
	addr := flag.String("db_address", "", "database address, such as: localhost:3306")
	flag.Parse()

	cfg := &mysql.Config{
		User:                 *user,
		Passwd:               *pswd,
		Net:                  "tcp",
		Addr:                 *addr,
		AllowNativePasswords: true,
	}
	db, err := sf.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterSQLFlowServer(s, server.NewServer(sf.Run, db))
	// Register reflection service on gRPC server.
	reflection.Register(s)
	log.Println("Server Started at", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
