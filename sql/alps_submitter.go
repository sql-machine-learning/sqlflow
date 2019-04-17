package sql

import (
	"fmt"
	"io"
	"strconv"
)

type GraphConfig struct {
	TableName string
	Fields    []string
	X         []FeatureSpec
	Y         FeatureSpec
}

type FeatureSpec struct {
	FeatureName string
	IsSparse    bool
	Shape       []int
	DType       string
}

type NumericColumn struct {
	Key   string
	Shape int
}

type BucketColumn struct {
	SourceColumn NumericColumn
	Boundaries   []int
}

type CrossColumn struct {
	Keys           []interface{}
	HashBucketSize int
}

func resolveFeatureSpec(el *exprlist) (*FeatureSpec, error) {
	headExpr := (*el)[0].val
	if headExpr == "DENSE" {
		shape, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, err
		}
		return &FeatureSpec{
			FeatureName: (*el)[1].val,
			IsSparse:    false,
			Shape:       []int{shape},
			DType:       "double"}, nil
	} else if headExpr == "SPARSE" {
		shape, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, err
		}
		return &FeatureSpec{
			FeatureName: (*el)[1].val,
			IsSparse:    false,
			Shape:       []int{shape},
			DType:       "double"}, nil
	}
	return nil, fmt.Errorf("not supported encoding type: %s", headExpr)
}

func resolveExpr(e *expr) (string, *exprlist) {
	if e.val != "" {
		return e.val, nil
	} else {
		return "", &e.sexp
	}
}

func resolveExprList(el *exprlist) (interface{}, error) {
	headName, headExprList := resolveExpr((*el)[0])
	if headName == "" {
		return resolveExprList(headExprList)
	} else {
		switch headName {
		case "DENSE":
			return resolveFeatureSpec(el)
		case "SPARSE":
			return resolveFeatureSpec(el)
		case "NUMERIC":
			shape, err := strconv.Atoi((*el)[2].val)
			if err != nil {
				return nil, err
			}
			return &NumericColumn{
				Key:   (*el)[1].val,
				Shape: shape}, nil
		case "BUCKET":
			sourceExprList := (*el)[1].sexp
			boundariesExprList := (*el)[2].sexp
			source, err := resolveExprList(&sourceExprList)
			if err != nil {
				return nil, err
			}
			boundaries, err := resolveExprList(&boundariesExprList)
			if err != nil {
				return nil, err
			}
			return &BucketColumn{
				SourceColumn: *source.(*NumericColumn),
				Boundaries:   transformToIntList(boundaries.([]interface{}))}, nil
		case "CROSS":
			keysExpr := (*el)[1].sexp
			bucketSize, _ := strconv.Atoi((*el)[2].val)
			keys, err := resolveExprList(&keysExpr)
			if err != nil {
				return nil, err
			}
			return &CrossColumn{
				Keys:           keys.([]interface{}),
				HashBucketSize: bucketSize}, nil
		case "square":
			var list []interface{}
			for idx, expr := range *el {
				if idx > 0 {
					val, sexp := resolveExpr(expr)
					if sexp == nil {
						val, _ := strconv.Atoi(val)
						list = append(list, val)
					} else {
						value, err := resolveExprList(sexp)
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
	}
	return nil, nil
}

func transformToIntList(interfaceList []interface{}) []int {
	var b []int
	for i := range interfaceList {
		b = append(b, interfaceList[i].(int))
	}
	return b
}

func resolveTrainColumns(el *exprlist) ([]interface{}, error) {
	var list []interface{}
	for _, expr := range *el {
		result, err := resolveExprList(&expr.sexp)
		if err != nil {
			return nil, err
		}
		list = append(list, result)
	}
	return list, nil
}

func genALPS(w io.Writer, pr *extendedSelect, fts fieldTypes, db *DB) error {
	// TODO
	return nil
}
