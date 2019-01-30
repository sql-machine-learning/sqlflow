//go:generate docker run --rm -v $PWD:/work -w /work grpc/go:1.0 protoc sqlflow.proto --go_out=plugins=grpc:.

package sqlflowserver

import (
	"fmt"
	"log"
	"strings"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
)

type Server struct{}

// streaming RPC server
func (*Server) Run(req *RunRequest, stream SQLFlow_RunServer) error {
	slct := req.Sql
	log.Printf("Received %s\n", slct)

	// TODO(tony): use a more robust criteria
	if strings.Contains(slct, "TRAIN") || strings.Contains(slct, "PREDICT") {
		return runExtendedSQL(slct, stream)
	}

	return runStandardSQL(slct, stream)
}

func wrapRow(row []interface{}) (*Table_Row, error) {
	wrappedRow := &Table_Row{}
	for _, element := range row {
		switch e := element.(type) {
		case int64:
			x, err := ptypes.MarshalAny(&wrappers.Int64Value{Value: e})
			if err != nil {
				return nil, err
			}
			wrappedRow.Data = append(wrappedRow.Data, x)
		default:
			return nil, fmt.Errorf("can convert type %#v to protobuf.Any", element)
		}
	}

	return wrappedRow, nil
}

// runStandardSQL sends
// | X  | Y  |
// |----|----|
// | 42 | 42 |
// | 42 | 42 |
// ...
func runStandardSQL(slct string, stream SQLFlow_RunServer) error {
	numSends := len(slct)
	for i := 0; i < numSends; i++ {
		table := &Table{}
		table.ColumnNames = []string{"X", "Y"}
		for i := 0; i < 2; i++ {
			row, err := wrapRow([]interface{}{interface{}(int64(42)), interface{}(int64(42))})
			if err != nil {
				return err
			}
			table.Rows = append(table.Rows, row)
		}
		res := &RunResponse{
			Response: &RunResponse_Table{
				Table: table}}
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
