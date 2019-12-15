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

func newZeroValue(t reflect.Type) interface{} {
	return reflect.New(t).Interface()
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
	case *bool, *string, *int, *int8, *int16, *int32, *int64, *uint, *uint8, *uint16, *uint32, *uint64, *float32, *float64:
		return val, nil
	default:
		return nil, fmt.Errorf("unrecognized field type %v", v)
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
