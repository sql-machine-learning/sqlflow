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

package codegen

import (
	"fmt"
	"strings"

	"sqlflow.org/sqlflow/go/ir"
)

func toModuleDataType(dtype int, module string) (string, error) {
	switch dtype {
	case ir.Int:
		return fmt.Sprintf("%s.dtypes.int64", module), nil
	case ir.Float:
		return fmt.Sprintf("%s.dtypes.float32", module), nil
	case ir.String:
		return fmt.Sprintf("%s.dtypes.string", module), nil
	default:
		return "", fmt.Errorf("unsupport dtype: %d", dtype)
	}
}

// TODO(sneaxiy): XGBoost does not support some feature columns, such as EMBEDDING.
// For better error message, we should find a better way to distinguish whether the
// module is TensorFlow or XGBoost.
func isXGBoostModule(module string) bool {
	return strings.HasPrefix(module, "xgboost")
}

// GenerateFeatureColumnCode generates feature column code for both TensorFlow and XGBoost models
func GenerateFeatureColumnCode(fc ir.FeatureColumn, module string) (string, error) {
	switch c := fc.(type) {
	case *ir.NumericColumn:
		shapeStr, err := ir.MarshalToJSONString(c.FieldDesc.Shape)
		if err != nil {
			return "", err
		}
		if module == "tf" {
			dtype, err := toModuleDataType(c.FieldDesc.DType, module)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s.feature_column.numeric_column(\"%s\", shape=%s, dtype=%s)",
				module,
				c.FieldDesc.Name,
				shapeStr,
				dtype), nil
		}
		return fmt.Sprintf("%s.feature_column.numeric_column(\"%s\", shape=%s)",
			module,
			c.FieldDesc.Name,
			shapeStr), nil

	case *ir.BucketColumn:
		sourceCode, err := GenerateFeatureColumnCode(c.SourceColumn, module)
		if err != nil {
			return "", err
		}
		boundariesStr, err := ir.MarshalToJSONString(c.Boundaries)
		if err != nil {
			return "", nil
		}
		return fmt.Sprintf(
			"%s.feature_column.bucketized_column(%s, boundaries=%s)",
			module,
			sourceCode, boundariesStr), nil
	case *ir.CategoryIDColumn:
		fm := c.GetFieldDesc()[0]
		if len(fm.Vocabulary) > 0 {
			vocabList := []string{}
			for k := range fm.Vocabulary {
				vocabList = append(vocabList, fmt.Sprintf("\"%s\"", k))
			}
			vocabCode := strings.Join(vocabList, ",")
			return fmt.Sprintf("%s.feature_column.categorical_column_with_vocabulary_list(key=\"%s\", vocabulary_list=[%s])",
				module, c.FieldDesc.Name, vocabCode), nil
		}
		return fmt.Sprintf("%s.feature_column.categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
			module, c.FieldDesc.Name, c.BucketSize), nil
	case *ir.SeqCategoryIDColumn:
		if isXGBoostModule(module) {
			return "", fmt.Errorf("SEQ_CATEGORY_ID is not supported in XGBoost models")
		}
		return fmt.Sprintf("%s.feature_column.sequence_categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
			module, c.FieldDesc.Name, c.BucketSize), nil
	case *ir.CategoryHashColumn:
		// FIXME(typhoonzero): do we need to support dtype other than int64?
		dtype, err := toModuleDataType(c.FieldDesc.DType, module)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.feature_column.categorical_column_with_hash_bucket(key=\"%s\", hash_bucket_size=%d, dtype=%s)",
			module, c.FieldDesc.Name, c.BucketSize, dtype), nil
	case *ir.WeightedCategoryColumn:
		sourceCode, err := GenerateFeatureColumnCode(c.CategoryColumn, module)
		if err != nil {
			return "", err
		}
		// automatically generate featurename_key as the weight key
		return fmt.Sprintf("%s.feature_column.weighted_categorical_column(categorical_column=%s, weight_feature_key=\"%s_weight\")",
			module, sourceCode, c.CategoryColumn.GetFieldDesc()[0].Name), nil
	case *ir.CrossColumn:
		if isXGBoostModule(module) {
			return "", fmt.Errorf("CROSS is not supported in XGBoost models")
		}

		var keysGenerated = make([]string, len(c.Keys))
		for idx, key := range c.Keys {
			if c, ok := key.(ir.FeatureColumn); ok {
				if nc, ok := c.(*ir.NumericColumn); ok {
					size := 1
					for s := range nc.FieldDesc.Shape {
						size *= s
					}

					if size > 1 {
						return "", fmt.Errorf("CROSS does not support shape not equal to 1")
					}
					keysGenerated[idx] = fmt.Sprintf("\"%s\"", nc.FieldDesc.Name)
				} else {
					code, err := GenerateFeatureColumnCode(c, module)
					if err != nil {
						return "", err
					}
					keysGenerated[idx] = code
				}
			} else if strKey, ok := key.(string); ok {
				keysGenerated[idx] = fmt.Sprintf("\"%s\"", strKey)
			} else {
				return "", fmt.Errorf("field in cross column is not a FeatureColumn or string type: %v", key)
			}
		}
		return fmt.Sprintf(
			"%s.feature_column.crossed_column([%s], hash_bucket_size=%d)",
			module, strings.Join(keysGenerated, ","), c.HashBucketSize), nil
	case *ir.EmbeddingColumn:
		if isXGBoostModule(module) {
			return "", fmt.Errorf("EMBEDDING is not supported in XGBoost models")
		}

		sourceCode, err := GenerateFeatureColumnCode(c.CategoryColumn, module)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.feature_column.embedding_column(%s, dimension=%d, combiner=\"%s\")",
			module, sourceCode, c.Dimension, c.Combiner), nil
	case *ir.IndicatorColumn:
		sourceCode, err := GenerateFeatureColumnCode(c.CategoryColumn, module)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.feature_column.indicator_column(%s)", module, sourceCode), nil
	default:
		return "", fmt.Errorf("unsupported feature column type %T on %v", c, c)
	}
}
