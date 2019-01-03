//go:generate protoc sqlflow.proto --go_out=plugins=grpc:.

package sqlflowserver

import (
	"fmt"
	"strings"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
)

type Server struct{}

// streaming RPC server
func (*Server) Run(req *RunRequest, stream SQLFlow_RunServer) error {
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
func runStandardSQL(slct string, stream SQLFlow_RunServer) error {
	numSends := len(slct)
	for i := 0; i < numSends; i++ {
		content := make(map[string]*Columns_Column)
		content["X"] = &Columns_Column{}
		content["Y"] = &Columns_Column{}
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
		res := &RunResponse{
			Response: &RunResponse_Columns{
				Columns: &Columns{
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
func runExtendedSQL(slct string, stream SQLFlow_RunServer) error {
	numSends := len(slct)
	for i := 0; i < numSends; i++ {
		content := []string{fmt.Sprintf("log %v", i)}
		res := &RunResponse{
			Response: &RunResponse_Messages{
				Messages: &Messages{
					Messages: content}}}
		if err := stream.Send(res); err != nil {
			return err
		}
	}
	return nil
}
