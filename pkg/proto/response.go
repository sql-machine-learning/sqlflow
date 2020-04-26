// Copyright 2020 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proto

import (
	fmt "fmt"
	"reflect"
	"time"

	proto "github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	pyts "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
)

// EncodeHead encode map to Response message
func EncodeHead(head map[string]interface{}) (*Response, error) {
	cn, ok := head["columnNames"]
	if !ok {
		return nil, fmt.Errorf("can't find field columnNames in head")
	}
	columnNames, ok := cn.([]string)
	if !ok {
		return nil, fmt.Errorf("head[\"columnNames\"] is of type %T, expected []string", cn)
	}
	return &Response{Response: &Response_Head{Head: &Head{ColumnNames: columnNames}}}, nil
}

// EncodeRow encodes interface slice to Response message
func EncodeRow(row []interface{}) (*Response, error) {
	encodedRow := &Row{}
	for _, element := range row {
		pm, err := encodePODType(element)
		if err != nil {
			return nil, err
		}
		any, err := ptypes.MarshalAny(pm)
		if err != nil {
			return nil, err
		}
		encodedRow.Data = append(encodedRow.Data, any)
	}
	return &Response{Response: &Response_Row{Row: encodedRow}}, nil
}

// EncodeMessage encodes string to Response message
func EncodeMessage(s string) (*Response, error) {
	return &Response{Response: &Response_Message{Message: &Message{Message: s}}}, nil
}

func encodePODType(val interface{}) (proto.Message, error) {
	switch v := val.(type) {
	case nil:
		return &Row_Null{}, nil
	case bool:
		return &wrappers.BoolValue{Value: v}, nil
	case int8, int16, int32:
		return &wrappers.Int32Value{Value: int32(reflect.ValueOf(val).Int())}, nil
	case int, int64:
		return &wrappers.Int64Value{Value: int64(reflect.ValueOf(val).Int())}, nil
	case uint8, uint16, uint32:
		return &wrappers.UInt32Value{Value: uint32(reflect.ValueOf(val).Uint())}, nil
	case uint, uint64:
		return &wrappers.UInt64Value{Value: uint64(reflect.ValueOf(val).Uint())}, nil
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
			Seconds: int64(v.Unix()),
			Nanos:   int32(v.Nanosecond())}, nil
	default:
		return nil, fmt.Errorf("Unknown Go type %#v to be converted into protobuf.Any", val)
	}
}

// DecodePODType decode a any.Any message to is unwrapped val
func DecodePODType(any *any.Any) (interface{}, error) {
	dval := ptypes.DynamicAny{}
	ptypes.UnmarshalAny(any, &dval)
	switch v := dval.Message.(type) {
	case nil:
		return nil, nil
	case *wrappers.BoolValue:
		return v.Value, nil
	case *wrappers.Int32Value:
		return v.Value, nil
	case *wrappers.Int64Value:
		return v.Value, nil
	case *wrappers.UInt32Value:
		return v.Value, nil
	case *wrappers.UInt64Value:
		return v.Value, nil
	case *wrappers.FloatValue:
		return v.Value, nil
	case *wrappers.DoubleValue:
		return v.Value, nil
	case *wrappers.StringValue:
		return v.Value, nil
	case *wrappers.BytesValue:
		return v.Value, nil
	case *pyts.Timestamp:
		return v.String(), nil
	default:
		return nil, fmt.Errorf("unknown Any: %v to convert to go", any)
	}
}
