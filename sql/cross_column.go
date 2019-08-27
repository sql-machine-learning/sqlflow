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
	"fmt"
	"strconv"
	"strings"
)

// TODO(uuleon) specify the hash_key if needed
type crossColumn struct {
	Keys           []interface{}
	HashBucketSize int
}

func (cc *crossColumn) GenerateCode() (string, error) {
	var keysGenerated = make([]string, len(cc.Keys))
	for idx, key := range cc.Keys {
		if c, ok := key.(featureColumn); ok {
			code, err := c.GenerateCode()
			if err != nil {
				return "", err
			}
			keysGenerated[idx] = code
			continue
		}
		if str, ok := key.(string); ok {
			keysGenerated[idx] = fmt.Sprintf("\"%s\"", str)
		} else {
			return "", fmt.Errorf("cross generate code error, key: %s", key)
		}
	}
	return fmt.Sprintf(
		"tf.feature_column.crossed_column([%s], hash_bucket_size=%d)",
		strings.Join(keysGenerated, ","), cc.HashBucketSize), nil
}

func (cc *crossColumn) GetDelimiter() string {
	return ""
}

func (cc *crossColumn) GetDtype() string {
	return ""
}

func (cc *crossColumn) GetKey() string {
	// NOTE: cross column is a feature on multiple column keys.
	return ""
}

func (cc *crossColumn) GetInputShape() string {
	// NOTE: return empty since crossed column input shape is already determined
	// by the two crossed columns.
	return ""
}

func resolveCrossColumn(el *exprlist) (*crossColumn, error) {
	if len(*el) != 3 {
		return nil, fmt.Errorf("bad CROSS expression format: %s", *el)
	}
	keysExpr := (*el)[1]
	keys, err := resolveExpression(keysExpr)
	if err != nil {
		return nil, err
	}
	if _, ok := keys.([]interface{}); !ok {
		return nil, fmt.Errorf("bad CROSS keys: %s", err)
	}
	bucketSize, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad CROSS bucketSize: %s, err: %s", (*el)[2].val, err)
	}
	return &crossColumn{
		Keys:           keys.([]interface{}),
		HashBucketSize: bucketSize}, nil
}
