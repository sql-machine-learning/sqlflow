//go:generate protoc -I proto proto/sqlflow.proto --go_out=plugins=grpc:proto
package server

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	pyts "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"

	pb "gitlab.alipay-inc.com/Arc/sqlflow/server/proto"
	sf "gitlab.alipay-inc.com/Arc/sqlflow/sql"
)

// NewServer returns a server instance
func NewServer(run func(string, *sql.DB) *sf.ExecutorChan, db *sql.DB) *server {
	return &server{run: run, db: db}
}

type server struct {
	run func(sql string, db *sql.DB) *sf.ExecutorChan
	db  *sql.DB
}

// Run implements `rpc Run (Request) returns (stream Response)`
func (s *server) Run(req *pb.Request, stream pb.SQLFlow_RunServer) error {
	ec := s.run(req.Sql, s.db)

	for r := range ec.Read() {
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
			ec.Close()
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
		pm, err := parse2Any(element)
		if err != nil {
			return nil, err
		}
		any, err := ptypes.MarshalAny(pm)
		if err != nil {
			return nil, err
		}
		encodedRow.Data = append(encodedRow.Data, any)
	}
	return &pb.Response{Response: &pb.Response_Row{Row: encodedRow}}, nil
}

func encodeMessage(s string) (*pb.Response, error) {
	return &pb.Response{Response: &pb.Response_Message{Message: &pb.Message{Message: s}}}, nil
}

func parse2Any(val interface{}) (proto.Message, error) {
	switch v := val.(type) {
	case nil:
		return &pb.Row_Null{}, nil
	case bool:
		return &wrappers.BoolValue{Value: v}, nil
	case int8:
		return &wrappers.Int32Value{Value: int32(v)}, nil
	case int16:
		return &wrappers.Int32Value{Value: int32(v)}, nil
	case int32:
		return &wrappers.Int32Value{Value: v}, nil
	case int:
		return &wrappers.Int64Value{Value: int64(v)}, nil
	case int64:
		return &wrappers.Int64Value{Value: v}, nil
	case uint8:
		return &wrappers.UInt32Value{Value: uint32(v)}, nil
	case uint16:
		return &wrappers.UInt32Value{Value: uint32(v)}, nil
	case uint32:
		return &wrappers.UInt32Value{Value: v}, nil
	case uint:
		return &wrappers.UInt64Value{Value: uint64(v)}, nil
	case uint64:
		return &wrappers.UInt64Value{Value: v}, nil
	case float32:
		return &wrappers.FloatValue{Value: v}, nil
	case float64:
		return &wrappers.DoubleValue{Value: v}, nil
	case string:
		return &wrappers.StringValue{Value: v}, nil
	case []byte:
		return &wrappers.BytesValue{Value: v}, nil
	case time.Time:
		return &pyts.Timestamp{
			Seconds: int64(v.Second()),
			Nanos:   int32(v.Nanosecond())}, nil
	default:
		return nil, fmt.Errorf("can't convert %#v to protobuf.Any", val)
	}
}
