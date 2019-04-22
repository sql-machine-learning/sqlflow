package sql

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/template"
)

const (
	sparse    = "SPARSE"
	numeric   = "NUMERIC"
	cross     = "CROSS"
	bucket    = "BUCKET"
	square    = "SQUARE"
	dense     = "DENSE"
	estimator = "ESTIMATOR"
)

type graphConfig struct {
	TableName string
	Fields    []string
	X         []featureSpec
	Y         featureSpec
}

type featureSpec struct {
	FeatureName string
	IsSparse    bool
	Shape       []int
	DType       string
	Delimiter   string
}

type FeatureColumn interface {
	GenerateCode() (string, error)
}

type NumericColumn struct {
	Key   string
	Shape int
}

type BucketColumn struct {
	SourceColumn *NumericColumn
	Boundaries   []int
}

type CrossColumn struct {
	Keys           []interface{}
	HashBucketSize int
}

type Attribute struct {
	FullName string
	Prefix   string
	Name     string
	Value    interface{}
}

func (nc *NumericColumn) GenerateCode() (string, error) {
	return fmt.Sprintf("tf.feature_column.numeric_column(\"%s\", shape=(%d,))", nc.Key, nc.Shape), nil
}

func (bc *BucketColumn) GenerateCode() (string, error) {
	sourceCode, _ := bc.SourceColumn.GenerateCode()
	return fmt.Sprintf(
		"tf.feature_column.bucketized_column(%s, boundaries=%s)",
		sourceCode,
		strings.Join(strings.Split(fmt.Sprint(bc.Boundaries), " "), ",")), nil
}

func (cc *CrossColumn) GenerateCode() (string, error) {
	var keysGenerated = make([]string, len(cc.Keys))
	for idx, key := range cc.Keys {
		if c, ok := key.(FeatureColumn); ok {
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

func (a *Attribute) GenerateCode() (string, error) {
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
	return "", fmt.Errorf("value of Attribute must be string or list of int, given %s", a.Value)
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
	delimiter, err := expression2string((*el)[3])
	if err != nil {
		return nil, fmt.Errorf("bad FeatureSpec delimiter: %s, err: %s", (*el)[1], err)
	}
	// TODO(uuleon): hard coded dtype(double) should be removed
	return &featureSpec{
		FeatureName: name,
		IsSparse:    isSparse,
		Shape:       []int{shape},
		DType:       "double",
		Delimiter:   delimiter}, nil
}

func resolveExpression(e interface{}) (interface{}, error) {
	if expr, ok := e.(*expr); ok {
		if expr.val != "" {
			return expr.val, nil
		} else {
			return resolveExpression(&expr.sexp)
		}
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
		return &NumericColumn{
			Key:   key,
			Shape: shape}, nil
	case bucket:
		if len(*el) != 3 {
			return nil, fmt.Errorf("bad BUCKET expression format: %s", *el)
		}
		sourceExprList := (*el)[1].sexp
		boundariesExprList := (*el)[2].sexp
		source, err := resolveExpression(&sourceExprList)
		if err != nil {
			return nil, err
		}
		if _, ok := source.(*NumericColumn); !ok {
			return nil, fmt.Errorf("key of BUCKET must be NUMERIC, which is %s", source)
		}
		boundaries, err := resolveExpression(&boundariesExprList)
		if err != nil {
			return nil, err
		}
		b, err := transformToIntList(boundaries.([]interface{}))
		if err != nil {
			return nil, fmt.Errorf("bad BUCKET boundaries: %s", err)
		}
		return &BucketColumn{
			SourceColumn: source.(*NumericColumn),
			Boundaries:   b}, nil
	case cross:
		if len(*el) != 3 {
			return nil, fmt.Errorf("bad CROSS expression format: %s", *el)
		}
		keysExpr := (*el)[1].sexp
		keys, err := resolveExpression(&keysExpr)
		if err != nil {
			return nil, err
		}
		bucketSize, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, fmt.Errorf("bad CROSS bucketSize: %s, err: %s", (*el)[2].val, err)
		}
		return &CrossColumn{
			Keys:           keys.([]interface{}),
			HashBucketSize: bucketSize}, nil
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
					} else {
						list = append(list, value)
					}
				}
			}
		}
		return list, nil
	default:
		return nil, fmt.Errorf("not supported expr in ALPS submitter: %s", headName)
	}
	return nil, nil
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

func expression2string(e interface{}) (string, error) {
	resolved, err := resolveExpression(e)
	if err != nil {
		return "", err
	}
	if str, ok := resolved.(string); !ok {
		return "", fmt.Errorf("expression expected to be string, actual: %s", resolved)
	} else {
		return str, nil
	}
}

func filter(attrs []*Attribute, prefix string) []*Attribute {
	ret := make([]*Attribute, 0)
	for _, a := range attrs {
		if strings.EqualFold(a.Prefix, prefix) {
			ret = append(ret, a)
		}
	}
	return ret
}

func resolveTrainColumns(columns *exprlist) ([]interface{}, error) {
	var list []interface{}
	for _, expr := range *columns {
		result, err := resolveExpression(expr)
		if err != nil {
			return nil, err
		}
		list = append(list, result)
	}
	return list, nil
}

func resolveTrainAttribute(attrs *attrs) ([]*Attribute, error) {
	var ret []*Attribute
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
		ret = append(ret, &Attribute{
			FullName: k,
			Prefix:   prefix,
			Name:     name,
			Value:    r})
	}
	return ret, nil
}

func generateFeatureColumnCode(fc interface{}) (string, error) {
	if fc, ok := fc.(FeatureColumn); ok {
		return fc.GenerateCode()
	} else {
		return "", fmt.Errorf("input is not FeatureColumn interface")
	}
}

func generateEstimatorCreator(estimator string, attrs []*Attribute) (string, error) {
	cl := make([]string, len(attrs))
	for idx, a := range attrs {
		code, err := a.GenerateCode()
		if err != nil {
			return "", err
		}
		cl[idx] = code
	}
	return fmt.Sprintf("tf.estimator.%s(%s)", estimator, strings.Join(cl, ",")), nil
}

func genALPS(w io.Writer, pr *extendedSelect, fts fieldTypes, db *DB) error {
	// TODO
	return nil
}

const alpsTemplateText = `
# coding: utf-8

import tensorflow as tf

from alps.feature import FeatureColumnsBuilder


class SQLFlowFCBuilder(FeatureColumnsBuilder):
	def build_feature_columns():
		return {{.FeatureColumnsCode}}

# TODO(uuleon) needs FeatureSpecs and ALPS client

`

var alpsTemplate = template.Must(template.New("feature_column").Parse(alpsTemplateText))
