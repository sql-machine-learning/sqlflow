//go:generate protoc -I proto proto/sqlflow.proto --go_out=plugins=grpc:proto
package server

import (
	"database/sql"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
	pb "gitlab.alipay-inc.com/Arc/sqlflow/server/proto"
)

// NewServer returns a server instance
func NewServer(run func(string, *sql.DB) chan interface{}, db *sql.DB) *server {
	return &server{run: run, db: db}
}

type server struct {
	run func(sql string, db *sql.DB) chan interface{}
	db  *sql.DB
}

// Run implements `rpc Run (Request) returns (stream Response)`
func (s *server) Run(req *pb.Request, stream pb.SQLFlow_RunServer) error {
	c := s.run(req.Sql, s.db)

	for r := range c {
		var res *pb.Response
		var err error
		switch s := r.(type) {
		case error:
			return s
		case map[string]interface{}:
			res, err = encodeHead(s)
		case []interface{}:
			res, err = encodeRow(s)
		case string:
			res, err = encodeMessage(s)
		default:
			return fmt.Errorf("unrecognize run channel return type %#v", s)
		}
		if err != nil {
			return err
		}
		if err := stream.Send(res); err != nil {
			// FIXME(tony): notify and exit sqlflow.Run
			return err
		}
	}

	return nil
}

func encodeHead(head map[string]interface{}) (*pb.Response, error) {
	cn, ok := head["columnNames"]
	if !ok {
		return nil, fmt.Errorf("can't find field columnNames in head")
	}
	columnNames, ok := cn.([]string)
	if !ok {
		return nil, fmt.Errorf("head[\"columnNames\"] is of type %T, expected []string", cn)
	}
	return &pb.Response{Response: &pb.Response_Head{Head: &pb.Head{ColumnNames: columnNames}}}, nil
}

func encodeRow(row []interface{}) (*pb.Response, error) {
	encodedRow := &pb.Row{}
	for _, element := range row {
		switch e := element.(type) {
		// TODO(tony): support more types
		case int64:
			x, err := ptypes.MarshalAny(&wrappers.Int64Value{Value: e})
			if err != nil {
				return nil, err
			}
			encodedRow.Data = append(encodedRow.Data, x)
		default:
			return nil, fmt.Errorf("can convert %#v to protobuf.Any", element)
		}
	}

	return &pb.Response{Response: &pb.Response_Row{Row: encodedRow}}, nil
}

func encodeMessage(s string) (*pb.Response, error) {
	return &pb.Response{Response: &pb.Response_Message{Message: &pb.Message{Message: s}}}, nil
}
