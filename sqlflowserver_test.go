package sqlflowserver

import (
	"fmt"
	"log"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	testStandardSQL = "SELECT ..."
	testExtendedSQL = "SELECT ... TRAIN ..."
)

func TestStandardSQL(t *testing.T) {
	a := assert.New(t)
	a.True(true)
}

func TestExtendedSQL(t *testing.T) {
	a := assert.New(t)
	a.True(true)
}

func startServer() {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	fmt.Println("Using port:", listener.Addr().(*net.TCPAddr).Port)

	s := grpc.NewServer()
	RegisterSQLFlowServer(s, &Server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)
	log.Println("Server Started at", listener.Addr().(*net.TCPAddr).Port)
	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func init() {
	go startServer()

	os.Exit()
}
