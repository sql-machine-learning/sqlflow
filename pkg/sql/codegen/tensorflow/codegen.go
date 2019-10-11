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

package tensorflow

import (
	"strings"

	"sqlflow.org/sqlflow/pkg/sql/codegen"
)

func intArrayToJsonString(ia []int) string {
	return strings.Join(strings.Split(fmt.Sprint(ia), " "), ",")
}

func generateFeatureColumnCode(fc codegen.FeatureColumn) (string, error) {
	switch c := fc.(type) {
	case *codegen.NumericColumn:
		nc := fc.(*codegen.NumericColumn)
		return fmt.Sprintf("tf.feature_column.numeric_column(\"%s\", shape=%s)",
			nc.FieldMeta.Name,
			intArrayToJsonString(nc.FieldMeta.Shape)), nil
	case *codegen.BucketColumn:
		bc := fc.(*codegen.BucketColumn)
		sourceCode, err := generateFeatureColumnCode(bc.SourceColumn)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(
			"tf.feature_column.bucketized_column(%s, boundaries=%s)",
			sourceCode,
			intArrayToJsonString(bc.Boundaries)), nil
	case *codegen.CategoryIDColumn:
		cc := fc.(*codegen.CategoryIDColumn)
		return fmt.Sprintf("tf.feature_column.categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
			cc.FieldMeta.Name, cc.BucketSize), nil
	case *codegen.SeqCategoryIDColumn:
		cc := fc.(*codegen.SeqCategoryIDColumn)
		return fmt.Sprintf("tf.feature_column.sequence_categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
			cc.FieldMeta.Name, cc.BucketSize), nil
	case *codegen.CrossColumn:
		cc := fc.(*codegen.CrossColumn)
		var keysGenerated = make([]string, len(cc.Keys))
		for idx, key := range cc.Keys {
			if c, ok := key.(FeatureColumn); ok {
				code, err := generateFeatureColumnCode(c)
				if err != nil {
					return "", err
				}
				keysGenerated[idx] = code
			} else {
				return "", fmt.Errorf("field in cross column is not a FeatureColumn type: %v", key)
			}
		}
		return fmt.Sprintf(
			"tf.feature_column.crossed_column([%s], hash_bucket_size=%d)",
			strings.Join(keysGenerated, ","), cc.HashBucketSize), nil
	case *codegen.EmbeddingColumn:
		ec := fc.(*codegen.EmbeddingColumn)
		catColumn, ok := ec.CategoryColumn.(codegen.FeatureColumn)
		if !ok {
			return "", fmt.Errorf("embedding generate code error, input is not featureColumn: %s", ec.CategoryColumn)
		}
		sourceCode, err := generateFeatureColumnCode(catColumn)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("tf.feature_column.embedding_column(%s, dimension=%d, combiner=\"%s\")",
			sourceCode, ec.Dimension, ec.Combiner), nil
	default:
		return "", fmt.Errorf("unsupported feature column type %T on %v", c, c)
	}
}

// Train generates a Python program for train a TensorFlow model.
func Train(ir codegen.TrainIR) (string, error) {
	trainParams := make(map[string]interface{})
	modelParams := make(map[string]interface{})
	for attrKey, attr := range ir.Attributes {
		if strings.HasPrefix(attrKey, "train.") {
			trainParams[strings.Replace(attrKey, "train.", "", 1)] = attr
		}
		if strings.HasPrefix(attrKey, "model.") {
			modelParams[strings.Replace(attrKey, "model.", "", 1)] = attr
		}
	}

	featureColumnsCode := make(map[string][]string)
	for target, fcList := range ir.Features {
		for _, fc := range fcList {
			fcCode := 
		}
	}

	filler := trainFiller{
		DataSource:       ir.DataSource,
		TrainSelect:      ir.Select,
		ValidationSelect: ir.ValidationSelect,
	}
	return "", nil
}
