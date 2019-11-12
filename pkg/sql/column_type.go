// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

package sql

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// hive column type ends with _TYPE
const hiveCTypeSuffix = "_TYPE"

var (
	// Column types: https://golang.org/pkg/database/sql/#Rows.Scan
	sqlNullBool    = reflect.TypeOf(sql.NullBool{})
	sqlNullInt64   = reflect.TypeOf(sql.NullInt64{})
	sqlNullFloat64 = reflect.TypeOf(sql.NullFloat64{})
	sqlRawBytes    = reflect.TypeOf(sql.RawBytes{})
	sqlNullString  = reflect.TypeOf(sql.NullString{})
	mysqlNullTime  = reflect.TypeOf(mysql.NullTime{})
	// builtin type supports sql like `select 1;` or  `select count(*) from ...`
	builtIntBytes  = reflect.TypeOf([]byte(""))
	builtinString  = reflect.TypeOf(string(""))
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
	builtinTime    = reflect.TypeOf(time.Time{})
)

func CreateByType(rt reflect.Type) (interface{}, error) {
	return createByType(rt)
}
func createByType(rt reflect.Type) (interface{}, error) {
	switch rt {
	case sqlNullBool:
		return new(sql.NullBool), nil
	case sqlNullInt64:
		return new(sql.NullInt64), nil
	case sqlNullFloat64:
		return new(sql.NullFloat64), nil
	case sqlRawBytes:
		return new(sql.RawBytes), nil
	case sqlNullString:
		return new(sql.NullString), nil
	case mysqlNullTime:
		return new(mysql.NullTime), nil
	case builtinTime:
		return new(time.Time), nil
	case builtIntBytes:
		return new([]byte), nil
	case builtinString:
		return new(string), nil
	case builtinInt:
		return new(int), nil
	case builtinInt8:
		return new(int8), nil
	case builtinInt16:
		return new(int16), nil
	case builtinInt32:
		return new(int32), nil
	case builtinInt64:
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
	case builtinFloat64:
		return new(float64), nil
	default:
		return nil, fmt.Errorf("unrecognized column scan type %v", rt)
	}
}

func parseVal(val interface{}) (interface{}, error) {
	switch v := val.(type) {
	case *sql.NullBool:
		if (*v).Valid {
			return (*v).Bool, nil
		}
		return nil, nil
	case *sql.NullInt64:
		if (*v).Valid {
			return (*v).Int64, nil
		}
		return nil, nil
	case *sql.NullFloat64:
		if (*v).Valid {
			return (*v).Float64, nil
		}
		return nil, nil
	case *sql.RawBytes:
		if *v == nil {
			return nil, nil
		}
		return string(*v), nil
	case *sql.NullString:
		if (*v).Valid {
			return (*v).String, nil
		}
		return nil, nil
	case *mysql.NullTime:
		if (*v).Valid {
			return (*v).Time, nil
		}
		return nil, nil
	case *(time.Time):
		return *v, nil
	case *([]byte):
		if *v == nil {
			return nil, nil
		}
		return *v, nil
	case *bool:
		return *v, nil
	case *string:
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
	default:
		return nil, fmt.Errorf("unrecognized type %v", v)
	}
}

func universalizeColumnType(driverName, dialectType string) (string, error) {
	if driverName == "mysql" || driverName == "maxcompute" {
		if dialectType == "VARCHAR" {
			// FIXME(tony): MySQL driver DatabaseName doesn't include the type length of a field.
			// Hardcoded to 255 for now.
			// ref: https://github.com/go-sql-driver/mysql/blob/877a9775f06853f611fb2d4e817d92479242d1cd/fields.go#L87
			return "VARCHAR(255)", nil
		}
		return dialectType, nil
	} else if driverName == "hive" {
		if strings.HasSuffix(dialectType, hiveCTypeSuffix) {
			return dialectType[:len(dialectType)-len(hiveCTypeSuffix)], nil
		}
		// In hive, capacity is also needed when define a VARCHAR field, so we replace it with STRING.
		if dialectType == "VARCHAR" {
			return "STRING", nil
		}
		return dialectType, nil
	}
	return "", fmt.Errorf("not support driver:%s", driverName)
}
