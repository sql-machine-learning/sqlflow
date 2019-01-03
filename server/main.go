package main

import (
	"fmt"
	"log"
	"net"
	"strings"

	pb "github.com/wangkuiyi/sqlflowserver"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	port = ":50051"
)

type server struct{}

// streaming RPC server
func (*server) Run(req *pb.RunRequest, stream pb.SQLFlow_RunServer) error {
	slct := req.Sql

	if strings.Contains(slct, "TRAIN") || strings.Contains(slct, "PREDICT") {
		return runExtendedSQL(slct, stream)
	}

	return runStandardSQL(slct, stream)
}

// runStandardSQL sends
// 	{"X": {0, 0, 0, 0}, "Y": {0, 0, 0, 0}}
// 	{"X": {1, 1, 1, 1}, "Y": {1, 1, 1, 1}}
// 	{"X": {2, 2, 2, 2}, "Y": {2, 2, 2, 2}}
//	...
// 	{"X": {N, N, N, N}, "Y": {N, N, N, N}}
func runStandardSQL(slct string, stream pb.SQLFlow_RunServer) error {
	numSends := len(slct)
	for i := 0; i < numSends; i++ {
		content := make(map[string]*pb.Columns_Column)
		content["X"] = &pb.Columns_Column{}
		content["Y"] = &pb.Columns_Column{}
		for j := 0; j < 4; j++ {
			x, err := ptypes.MarshalAny(&wrappers.Int64Value{Value: int64(i)})
			if err != nil {
				return err
			}
			content["X"].Data = append(content["X"].Data, x)
			y, err := ptypes.MarshalAny(&wrappers.Int64Value{Value: int64(i)})
			if err != nil {
				return err
			}
			content["Y"].Data = append(content["Y"].Data, y)
		}
		res := &pb.RunResponse{
			Response: &pb.RunResponse_Columns{
				Columns: &pb.Columns{
					Columns: content}}}
		if err := stream.Send(res); err != nil {
			return err
		}
	}

	return nil
}

// runExtendedSQL sends
//	log 0
//	log 1
//	log 2
//	...
//	log N
func runExtendedSQL(slct string, stream pb.SQLFlow_RunServer) error {
	numSends := len(slct)
	for i := 0; i < numSends; i++ {
		content := []string{fmt.Sprintf("log %v", i)}
		res := &pb.RunResponse{
			Response: &pb.RunResponse_Messages{
				Messages: &pb.Messages{
					Messages: content}}}
		if err := stream.Send(res); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterSQLFlowServer(s, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
