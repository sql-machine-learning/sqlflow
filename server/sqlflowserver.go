// Package server implements a gRPC proxy server of SQLFlow engines
package server

//go:generate docker run --rm -v $PWD:/work -w /work grpc/go:1.0 protoc sqlflow.proto --go_out=plugins=grpc:.

import (
	"fmt"
	"log"
	"strings"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
)

// Server instance
type Server struct{}

// Query executes a SQL statement
//
// SQL statements like `SELECT ...`, `DESCRIBE ...` returns a rowset.
// The rowset might be big. In such cases, Query returns a stream
// of RunResponse
func (*Server) Query(req *Request, stream SQLFlow_QueryServer) error {
	slct := req.Sql
	log.Printf("Received %s\n", slct)

	return runStandardSQL(slct, stream)
}

// Execute executes a SQL statement
//
// SQL statements like `USE database`, `DELETE` returns only a success
// message.
//
// SQL statement like `SELECT ... TRAIN/PREDICT ...` returns a stream of
// messages which indicates the training/predicting progress
func (*Server) Execute(req *Request, stream SQLFlow_ExecuteServer) error {
	slct := req.Sql
	log.Printf("Received %s\n", slct)

	// ExtendedSQL such as SELECT ... TRAIN/PREDICT
	if strings.Contains(slct, "TRAIN") || strings.Contains(slct, "PREDICT") {
		return runExtendedSQL(slct, stream)
	}

	// SQL such as INSERT, DELETE, CREATE
	return stream.Send(&Messages{Messages: []string{"Query OK, 0 rows affected (0.06 sec)"}})
}

// runStandardSQL sends
// | X  | Y  |
// |----|----|
// | 42 | 42 |
// | 42 | 42 |
// ...
func runStandardSQL(slct string, stream SQLFlow_QueryServer) error {
	numSends := len(slct)
	for i := 0; i < numSends; i++ {
		rowset := &RowSet{}
		rowset.ColumnNames = []string{"X", "Y"}
		for i := 0; i < 2; i++ {
			row, err := wrapRow([]interface{}{interface{}(int64(42)), interface{}(int64(42))})
			if err != nil {
				return err
			}
			rowset.Rows = append(rowset.Rows, row)
		}
		if err := stream.Send(rowset); err != nil {
			return err
		}
	}

	return nil
}

func wrapRow(row []interface{}) (*RowSet_Row, error) {
	wrappedRow := &RowSet_Row{}
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

// runExtendedSQL sends
//	log 0
//	log 1
//	log 2
//	...
//	log N
func runExtendedSQL(slct string, stream SQLFlow_ExecuteServer) error {
	numSends := len(slct)
	for i := 0; i < numSends; i++ {
		content := []string{fmt.Sprintf("log %v", i)}
		res := &Messages{Messages: content}
		if err := stream.Send(res); err != nil {
			return err
		}
	}
	return nil
}
