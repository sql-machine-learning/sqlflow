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

package columns

import (
	"fmt"
	"strings"
)

// CrossColumn is the wapper of `tf.feature_column.crossed_column`
// TODO(uuleon) specify the hash_key if needed
type CrossColumn struct {
	Keys           []interface{}
	HashBucketSize int
}

// GenerateCode implements the FeatureColumn interface.
func (cc *CrossColumn) GenerateCode(cs *ColumnSpec) ([]string, error) {
	var keysGenerated = make([]string, len(cc.Keys))
	for idx, key := range cc.Keys {
		if c, ok := key.(FeatureColumn); ok {
			codeList, err := c.GenerateCode(cs)
			if err != nil {
				return []string{}, err
			}
			if len(codeList) > 1 {
				return []string{}, fmt.Errorf("cross column does not support crossing multi feature column types")
			}
			keysGenerated[idx] = codeList[0]
			continue
		}
		if str, ok := key.(string); ok {
			keysGenerated[idx] = fmt.Sprintf("\"%s\"", str)
		} else {
			return []string{}, fmt.Errorf("cross generate code error, key: %s", key)
		}
	}
	return []string{fmt.Sprintf(
		"tf.feature_column.crossed_column([%s], hash_bucket_size=%d)",
		strings.Join(keysGenerated, ","), cc.HashBucketSize)}, nil
}

// GetKey implements the FeatureColumn interface.
func (cc *CrossColumn) GetKey() string {
	// NOTE: cross column is a feature on multiple column keys.
	return ""
}

// GetDelimiter implements the FeatureColumn interface.
func (cc *CrossColumn) GetDelimiter() string {
	return ""
}

// GetDtype implements the FeatureColumn interface.
func (cc *CrossColumn) GetDtype() string {
	return ""
}

// GetInputShape implements the FeatureColumn interface.
func (cc *CrossColumn) GetInputShape() string {
	// NOTE: return empty since crossed column input shape is already determined
	// by the two crossed columns.
	return ""
}

// GetColumnType implements the FeatureColumn interface.
func (cc *CrossColumn) GetColumnType() int {
	return ColumnTypeCross
}
