package sql

import (
	"database/sql"
	"fmt"
	"reflect"
)

var (
	// Column types: https://golang.org/pkg/database/sql/#Rows.Scan
	sqlNullBool    = reflect.TypeOf(sql.NullBool{})
	sqlNullString  = reflect.TypeOf(sql.NullString{})
	sqlRawBytes    = reflect.TypeOf(sql.RawBytes{})
	sqlNullInt64   = reflect.TypeOf(sql.NullInt64{})
	sqlNullFloat64 = reflect.TypeOf(sql.NullFloat64{})
	builtIntBytes  = reflect.TypeOf([]byte(""))
	builtinInt     = reflect.TypeOf(int(0))
	builtinInt8    = reflect.TypeOf(int8(0))
	builtinInt16   = reflect.TypeOf(int16(0))
	builtinInt32   = reflect.TypeOf(int32(0))
	builtinInt64   = reflect.TypeOf(int64(0))
	builtinUint    = reflect.TypeOf(uint(0))
	builtinUint8   = reflect.TypeOf(uint8(0))
	builtinUint16  = reflect.TypeOf(uint16(0))
	builtinUint32  = reflect.TypeOf(uint32(0))
	builtinUint64  = reflect.TypeOf(uint64(0))
	builtinFloat32 = reflect.TypeOf(float32(0))
	builtinFloat64 = reflect.TypeOf(float64(0))
)

func mmallocByType(rt reflect.Type) (interface{}, error) {
	switch rt {
	case sqlNullBool:
		return new(bool), nil
	case sqlNullString:
		return new(string), nil
	case sqlRawBytes:
		return new(sql.RawBytes), nil
	case builtIntBytes:
		return new([]byte), nil
	case builtinInt:
		return new(int), nil
	case builtinInt8:
		return new(int8), nil
	case builtinInt16:
		return new(int16), nil
	case builtinInt32:
		return new(int32), nil
	case sqlNullInt64, builtinInt64:
		return new(int64), nil
	case builtinUint:
		return new(uint), nil
	case builtinUint8:
		return new(uint8), nil
	case builtinUint16:
		return new(uint16), nil
	case builtinUint32:
		return new(uint32), nil
	case builtinUint64:
		return new(uint64), nil
	case builtinFloat32:
		return new(float32), nil
	case sqlNullFloat64, builtinFloat64:
		return new(float64), nil
	default:
		return nil, fmt.Errorf("unrecognized column scan type %v", rt)
	}
}

func parseVal(val interface{}) (interface{}, error) {
	switch v := val.(type) {
	case nil:
		return nil, nil
	case *bool:
		return *v, nil
	case *string:
		return *v, nil
	case *([]byte):
		if *v == nil {
			return nil, nil
		}
		return *v, nil
	case *int:
		return *v, nil
	case *int8:
		return *v, nil
	case *int16:
		return *v, nil
	case *int32:
		return *v, nil
	case *int64:
		return *v, nil
	case *uint:
		return *v, nil
	case *uint8:
		return *v, nil
	case *uint16:
		return *v, nil
	case *uint32:
		return *v, nil
	case *uint64:
		return *v, nil
	case *float32:
		return *v, nil
	case *float64:
		return *v, nil
	case *sql.RawBytes:
		if *v == nil {
			return nil, nil
		}
		return string(*v), nil
	default:
		return nil, fmt.Errorf("unrecogized type %v", v)
	}
}
