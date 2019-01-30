// To run this program:
//	go generate .. && go run main.go
//
package main

import (
	"context"
	"io"
	"log"
	"time"

	pb "github.com/wangkuiyi/sqlflowserver"
	"google.golang.org/grpc"
)

const (
	address     = "localhost:50051"
	standardSQL = "SELECT ..."
	extendedSQL = "SELECT ... TRAIN ..."
)

func run(c pb.SQLFlowClient, sql string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	r, err := c.Run(ctx, &pb.RunRequest{Sql: sql})
	if err != nil {
		log.Fatalf("%v", err)
	}

	for {
		res, err := r.Recv()
		if err != nil {
			if err != io.EOF {
				log.Println(err)
			}
			break
		}
		log.Println(res)
	}

}

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewSQLFlowClient(conn)

	log.Printf("Running %s", standardSQL)
	run(c, standardSQL)
	log.Printf("Running %s", extendedSQL)
	run(c, extendedSQL)
}
