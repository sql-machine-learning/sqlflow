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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type numericColumn struct {
	Key       string
	Shape     []int
	Delimiter string
	Dtype     string
}

func (nc *numericColumn) GenerateCode() (string, error) {
	return fmt.Sprintf("tf.feature_column.numeric_column(\"%s\", shape=%s)", nc.Key,
		strings.Join(strings.Split(fmt.Sprint(nc.Shape), " "), ",")), nil
}

func (nc *numericColumn) GetDelimiter() string {
	return nc.Delimiter
}

func (nc *numericColumn) GetDtype() string {
	return nc.Dtype
}

func (nc *numericColumn) GetKey() string {
	return nc.Key
}

func (nc *numericColumn) GetInputShape() string {
	jsonBytes, err := json.Marshal(nc.Shape)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

func (nc *numericColumn) GetColumnType() int {
	return columnTypeNumeric
}

func resolveNumericColumn(el *exprlist) (*numericColumn, error) {
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad NUMERIC expression format: %s", *el)
	}
	key, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad NUMERIC key: %s, err: %s", (*el)[1], err)
	}
	var shape []int
	intVal, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		list, _, err := resolveExpression((*el)[2])
		if err != nil {
			return nil, err
		}
		if list, ok := list.([]interface{}); ok {
			shape, err = transformToIntList(list)
			if err != nil {
				return nil, fmt.Errorf("bad NUMERIC shape: %s, err: %s", (*el)[2].val, err)
			}
		} else {
			return nil, fmt.Errorf("bad NUMERIC shape: %s, err: %s", (*el)[2].val, err)
		}
	} else {
		shape = append(shape, intVal)
	}
	return &numericColumn{
		Key:   key,
		Shape: shape,
		// FIXME(typhoonzero, tony): support config Delimiter and Dtype
		Delimiter: ",",
		Dtype:     "float32"}, nil
}
