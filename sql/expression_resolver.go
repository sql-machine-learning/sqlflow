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

const (
	sparse    = "SPARSE"
	numeric   = "NUMERIC"
	cross     = "CROSS"
	catID     = "CAT_ID"
	seqCatID  = "SEQ_CAT_ID"
	embedding = "EMBEDDING"
	bucket    = "BUCKET"
	square    = "SQUARE"
	dense     = "DENSE"
	estimator = "ESTIMATOR"
	comma     = "COMMA"
)

// featureColumn is an interface that all types of feature columns and
// attributes (WITH clause) should follow.
// featureColumn is used to generate feature column code.
type featureColumn interface {
	GenerateCode() (string, error)
	// Some feature columns accept input tensors directly, and the data
	// may be a tensor string like: 12,32,4,58,0,0
	GetDelimiter() string
	GetDtype() string
	GetKey() string
}

// featureSpec contains information to generate DENSE/SPARSE code
type featureSpec struct {
	FeatureName string
	IsSparse    bool
	Shape       []int
	DType       string
	Delimiter   string
}

type attribute struct {
	FullName string
	Prefix   string
	Name     string
	Value    interface{}
}

type numericColumn struct {
	Key   string
	Shape int
	Dtype string
}

type bucketColumn struct {
	SourceColumn *numericColumn
	Boundaries   []int
}

type crossColumn struct {
	Keys           []interface{}
	HashBucketSize int
}

type catIDColumn struct {
	Key        string
	BucketSize int
	Delimiter  string
	Dtype      string
}

type sequenceCatIDColumn struct {
	Key        string
	BucketSize int
	Delimiter  string
	Dtype      string
}

type embeddingColumn struct {
	CatColumn interface{}
	Dimension int
	Combiner  string
}

// resolveTrainColumns resolve columns from SQL statement,
// returns type string, featureColumn list or featureSpecs
func resolveTrainColumns(columns *exprlist) ([]featureColumn, map[string]*featureSpec, error) {
	var fsMap = make(map[string]*featureSpec)
	var fcList = make([]featureColumn, 0)
	for _, expr := range *columns {
		result, err := resolveExpression(expr)
		if err != nil {
			return nil, nil, err
		}
		if fs, ok := result.(*featureSpec); ok {
			fsMap[fs.FeatureName] = fs
			continue
		} else if c, ok := result.(featureColumn); ok {
			fcList = append(fcList, c)
		} else if s, ok := result.(string); ok {
			// simple string column, generate default feature column
			c := &numericColumn{
				Key:   s,
				Shape: 1,
				Dtype: "float32",
			}
			fcList = append(fcList, c)
		} else {
			return nil, nil, fmt.Errorf("not recgonized type: %s", result)
		}
	}
	return fcList, fsMap, nil
}

// resolveExpression resolve a SQLFlow expression to the actual value
// see: sql.y:241 for the defination of expression.
func resolveExpression(e interface{}) (interface{}, error) {
	if expr, ok := e.(*expr); ok {
		if expr.val != "" {
			return expr.val, nil
		}
		return resolveExpression(&expr.sexp)
	}

	el, ok := e.(*exprlist)
	if !ok {
		return nil, fmt.Errorf("input of resolveExpression must be `expr` or `exprlist`: %s", e)
	}

	headName := (*el)[0].val
	if headName == "" {
		return resolveExpression(&(*el)[0].sexp)
	}

	headName = strings.ToUpper(headName)

	switch headName {
	case dense:
		return resolveFeatureSpec(el, false)
	case sparse:
		return resolveFeatureSpec(el, true)
	case numeric:
		if len(*el) != 3 {
			return nil, fmt.Errorf("bad NUMERIC expression format: %s", *el)
		}
		key, err := expression2string((*el)[1])
		if err != nil {
			return nil, fmt.Errorf("bad NUMERIC key: %s, err: %s", (*el)[1], err)
		}
		shape, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, fmt.Errorf("bad NUMERIC shape: %s, err: %s", (*el)[2].val, err)
		}
		return &numericColumn{
			Key:   key,
			Shape: shape,
			// FIXME(typhoonzero): support config dtype
			Dtype: "float32"}, nil
	case bucket:
		if len(*el) != 3 {
			return nil, fmt.Errorf("bad BUCKET expression format: %s", *el)
		}
		sourceExprList := (*el)[1]
		boundariesExprList := (*el)[2]
		source, err := resolveExpression(sourceExprList)
		if err != nil {
			return nil, err
		}
		if _, ok := source.(*numericColumn); !ok {
			return nil, fmt.Errorf("key of BUCKET must be NUMERIC, which is %s", source)
		}
		boundaries, err := resolveExpression(boundariesExprList)
		if err != nil {
			return nil, err
		}
		if _, ok := boundaries.([]interface{}); !ok {
			return nil, fmt.Errorf("bad BUCKET boundaries: %s", err)
		}
		b, err := transformToIntList(boundaries.([]interface{}))
		if err != nil {
			return nil, fmt.Errorf("bad BUCKET boundaries: %s", err)
		}
		return &bucketColumn{
			SourceColumn: source.(*numericColumn),
			Boundaries:   b}, nil
	case cross:
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
	case catID:
		if len(*el) != 3 && len(*el) != 4 {
			return nil, fmt.Errorf("bad CAT_ID expression format: %s, len: %d", *el, len(*el))
		}
		key, err := expression2string((*el)[1])
		if err != nil {
			return nil, fmt.Errorf("bad CAT_ID key: %s, err: %s", (*el)[1], err)
		}
		bucketSize, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, fmt.Errorf("bad CAT_ID bucketSize: %s, err: %s", (*el)[2].val, err)
		}
		delimiter := ""
		if len(*el) == 4 {
			delimiter, err = resolveDelimiter((*el)[3].val)
			if err != nil {
				return nil, fmt.Errorf("bad CAT_ID delimiter: %s, %s", (*el)[3].val, err)
			}
		}
		return &catIDColumn{
			Key:        key,
			BucketSize: bucketSize,
			Delimiter:  delimiter,
			// TODO(typhoonzero): support config dtype
			Dtype: "int64"}, nil
	case seqCatID:
		if len(*el) != 3 && len(*el) != 4 {
			return nil, fmt.Errorf("bad CAT_ID expression format: %s", *el)
		}
		key, err := expression2string((*el)[1])
		if err != nil {
			return nil, fmt.Errorf("bad CAT_ID key: %s, err: %s", (*el)[1], err)
		}
		bucketSize, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, fmt.Errorf("bad CAT_ID bucketSize: %s, err: %s", (*el)[2].val, err)
		}
		delimiter := ""
		if len(*el) == 4 {
			delimiter, err = resolveDelimiter((*el)[3].val)
			if err != nil {
				return nil, fmt.Errorf("bad CAT_ID delimiter: %s, %s", (*el)[3].val, err)
			}
		}
		return &sequenceCatIDColumn{
			Key:        key,
			BucketSize: bucketSize,
			Delimiter:  delimiter,
			Dtype:      "int64"}, nil
	case embedding:
		if len(*el) != 4 {
			return nil, fmt.Errorf("bad EMBEDDING expression format: %s, len: %d", *el, len(*el))
		}
		sourceExprList := (*el)[1]
		source, err := resolveExpression(sourceExprList)
		if err != nil {
			return nil, err
		}
		// TODO(uuleon) support other kinds of categorical column in the future
		var catColumn interface{}
		catColumn, ok := source.(*catIDColumn)
		if !ok {
			catColumn, ok = source.(*sequenceCatIDColumn)
			if !ok {
				return "", fmt.Errorf("key of EMBEDDING must be categorical column")
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
		return &embeddingColumn{
			CatColumn: catColumn,
			Dimension: dimension,
			Combiner:  combiner}, nil
	case square:
		var list []interface{}
		for idx, expr := range *el {
			if idx > 0 {
				if expr.sexp == nil {
					intVal, err := strconv.Atoi(expr.val)
					if err != nil {
						list = append(list, expr.val)
					} else {
						list = append(list, intVal)
					}
				} else {
					value, err := resolveExpression(&expr.sexp)
					if err != nil {
						return nil, err
					}
					list = append(list, value)
				}
			}
		}
		return list, nil
	default:
		return nil, fmt.Errorf("not supported expr: %s", headName)
	}
}

func resolveFeatureSpec(el *exprlist, isSparse bool) (*featureSpec, error) {
	if len(*el) != 4 {
		return nil, fmt.Errorf("bad FeatureSpec expression format: %s", *el)
	}
	name, err := expression2string((*el)[1])
	if err != nil {
		return nil, fmt.Errorf("bad FeatureSpec name: %s, err: %s", (*el)[1], err)
	}
	shape, err := strconv.Atoi((*el)[2].val)
	if err != nil {
		return nil, fmt.Errorf("bad FeatureSpec shape: %s, err: %s", (*el)[2].val, err)
	}
	unresolvedDelimiter, err := expression2string((*el)[3])
	if err != nil {
		return nil, fmt.Errorf("bad FeatureSpec delimiter: %s, err: %s", (*el)[1], err)
	}

	delimiter, err := resolveDelimiter(unresolvedDelimiter)
	if err != nil {
		return nil, err
	}
	// TODO(uuleon): hard coded dtype(float) should be removed
	return &featureSpec{
		FeatureName: name,
		IsSparse:    isSparse,
		Shape:       []int{shape},
		DType:       "float",
		Delimiter:   delimiter}, nil
}

func expression2string(e interface{}) (string, error) {
	resolved, err := resolveExpression(e)
	if err != nil {
		return "", err
	}
	if str, ok := resolved.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("expression expected to be string, actual: %s", resolved)
}

func (nc *numericColumn) GenerateCode() (string, error) {
	return fmt.Sprintf("tf.feature_column.numeric_column(\"%s\", shape=(%d,))", nc.Key, nc.Shape), nil
}

func (nc *numericColumn) GetDelimiter() string {
	return ""
}

func (nc *numericColumn) GetDtype() string {
	return nc.Dtype
}

func (nc *numericColumn) GetKey() string {
	return nc.Key
}

func (bc *bucketColumn) GenerateCode() (string, error) {
	sourceCode, _ := bc.SourceColumn.GenerateCode()
	return fmt.Sprintf(
		"tf.feature_column.bucketized_column(%s, boundaries=%s)",
		sourceCode,
		strings.Join(strings.Split(fmt.Sprint(bc.Boundaries), " "), ",")), nil
}

func (bc *bucketColumn) GetDelimiter() string {
	return ""
}

func (bc *bucketColumn) GetDtype() string {
	return ""
}

func (bc *bucketColumn) GetKey() string {
	return bc.SourceColumn.Key
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

func (cc *catIDColumn) GenerateCode() (string, error) {
	return fmt.Sprintf("tf.feature_column.categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
		cc.Key, cc.BucketSize), nil
}

func (cc *catIDColumn) GetDelimiter() string {
	return cc.Delimiter
}

func (cc *catIDColumn) GetDtype() string {
	return cc.Dtype
}

func (cc *catIDColumn) GetKey() string {
	return cc.Key
}

func (cc *sequenceCatIDColumn) GenerateCode() (string, error) {
	return fmt.Sprintf("tf.feature_column.sequence_categorical_column_with_identity(key=\"%s\", num_buckets=%d)",
		cc.Key, cc.BucketSize), nil
}

func (cc *sequenceCatIDColumn) GetDelimiter() string {
	return cc.Delimiter
}

func (cc *sequenceCatIDColumn) GetDtype() string {
	return cc.Dtype
}

func (cc *sequenceCatIDColumn) GetKey() string {
	return cc.Key
}

func (ec *embeddingColumn) GenerateCode() (string, error) {
	catColumn, ok := ec.CatColumn.(featureColumn)
	if !ok {
		return "", fmt.Errorf("embedding generate code error, input is not featureColumn: %s", ec.CatColumn)
	}
	sourceCode, err := catColumn.GenerateCode()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tf.feature_column.embedding_column(%s, dimension=%d, combiner=\"%s\")",
		sourceCode, ec.Dimension, ec.Combiner), nil
}

func (ec *embeddingColumn) GetDelimiter() string {
	return ec.CatColumn.(featureColumn).GetDelimiter()
}

func (ec *embeddingColumn) GetDtype() string {
	return ec.CatColumn.(featureColumn).GetDtype()
}

func (ec *embeddingColumn) GetKey() string {
	return ec.CatColumn.(featureColumn).GetKey()
}

func resolveTrainAttribute(attrs *attrs) ([]*attribute, error) {
	var ret []*attribute
	for k, v := range *attrs {
		subs := strings.SplitN(k, ".", 2)
		name := subs[len(subs)-1]
		prefix := ""
		if len(subs) == 2 {
			prefix = subs[0]
		}
		r, err := resolveExpression(v)
		if err != nil {
			return nil, err
		}
		ret = append(ret, &attribute{
			FullName: k,
			Prefix:   prefix,
			Name:     name,
			Value:    r})
	}
	return ret, nil
}

func resolveDelimiter(delimiter string) (string, error) {
	if strings.EqualFold(delimiter, comma) {
		return ",", nil
	}
	return "", fmt.Errorf("unsolved delimiter: %s", delimiter)
}

func (a *attribute) GenerateCode() (string, error) {
	if val, ok := a.Value.(string); ok {
		return fmt.Sprintf("%s=\"%s\"", a.Name, val), nil
	}
	if val, ok := a.Value.([]interface{}); ok {
		intList, err := transformToIntList(val)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s=%s", a.Name,
			strings.Join(strings.Split(fmt.Sprint(intList), " "), ",")), nil
	}
	return "", fmt.Errorf("value of attribute must be string or list of int, given %s", a.Value)
}

func transformToIntList(list []interface{}) ([]int, error) {
	var b = make([]int, len(list))
	for idx, item := range list {
		if intVal, ok := item.(int); ok {
			b[idx] = intVal
		} else {
			return nil, fmt.Errorf("type is not int: %s", item)
		}
	}
	return b, nil
}
