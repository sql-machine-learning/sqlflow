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
)

type embeddingColumn struct {
	CategoryColumn interface{}
	Dimension      int
	Combiner       string
	Initializer    string
}

func (ec *embeddingColumn) GetDelimiter() string {
	return ec.CategoryColumn.(featureColumn).GetDelimiter()
}

func (ec *embeddingColumn) GetDtype() string {
	return ec.CategoryColumn.(featureColumn).GetDtype()
}

func (ec *embeddingColumn) GetKey() string {
	return ec.CategoryColumn.(featureColumn).GetKey()
}

func (ec *embeddingColumn) GetInputShape() string {
	return ec.CategoryColumn.(featureColumn).GetInputShape()
}

func (ec *embeddingColumn) GetColumnType() int {
	return columnTypeEmbedding
}

func (ec *embeddingColumn) GenerateCode() (string, error) {
	catColumn, ok := ec.CategoryColumn.(featureColumn)
	if !ok {
		return "", fmt.Errorf("embedding generate code error, input is not featureColumn: %s", ec.CategoryColumn)
	}
	sourceCode, err := catColumn.GenerateCode()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tf.feature_column.embedding_column(%s, dimension=%d, combiner=\"%s\")",
		sourceCode, ec.Dimension, ec.Combiner), nil
}

func resolveEmbeddingColumn(el *exprlist) (*embeddingColumn, error) {
	if len(*el) != 4 && len(*el) != 5 {
		return nil, fmt.Errorf("bad EMBEDDING expression format: %s", *el)
	}
	sourceExprList := (*el)[1]
	var source featureColumn
	var err error
	if sourceExprList.typ == 0 {
		source, _, err = resolveColumn(&sourceExprList.sexp)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("key of EMBEDDING must be categorical column")
	}
	// TODO(uuleon) support other kinds of categorical column in the future
	var catColumn interface{}
	catColumn, ok := source.(*categoryIDColumn)
	if !ok {
		catColumn, ok = source.(*sequenceCategoryIDColumn)
		if !ok {
			return nil, fmt.Errorf("key of EMBEDDING must be categorical column")
		}
	}
	dimension, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad EMBEDDING dimension: %s, err: %s", (*el)[2].val, err)
	}
	combiner, err := expression2string((*el)[3])
	if err != nil {
		return nil, fmt.Errorf("bad EMBEDDING combiner: %s, err: %s", (*el)[3], err)
	}
	initializer := ""
	if len(*el) == 5 {
		initializer, err = expression2string((*el)[4])
		if err != nil {
			return nil, fmt.Errorf("bad EMBEDDING initializer: %s, err: %s", (*el)[4], err)
		}
	}
	return &embeddingColumn{
		CategoryColumn: catColumn,
		Dimension:      dimension,
		Combiner:       combiner,
		Initializer:    initializer}, nil
}
