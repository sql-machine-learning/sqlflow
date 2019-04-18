package sql

import (
	"fmt"
	"io"
	"strconv"
)

const (
	SPARSE  = "SPARSE"
	NUMERIC = "NUMERIC"
	CROSS   = "CROSS"
	BUCKET  = "BUCKET"
	SQUARE  = "square"
	DENSE   = "DENSE"
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

func resolveFeatureSpec(el *exprlist) (*featureSpec, error) {
	headExpr := (*el)[0].val
	if headExpr == DENSE {
		if len(*el) != 4 {
			return nil, fmt.Errorf("bad DENSE expression format: %s", *el)
		}
		if _, sexp := resolveExpr((*el)[1]); sexp != nil {
			return nil, fmt.Errorf("bad DENSE name: %s", *sexp)
		}
		shape, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, fmt.Errorf("bad DENSE shape: %s, err: %s", (*el)[2].val, err)
		}
		// TODO(uuleon): hard coded dtype(double) should be removed
		return &featureSpec{
			FeatureName: (*el)[1].val,
			IsSparse:    false,
			Shape:       []int{shape},
			DType:       "double",
			Delimiter:   (*el)[3].val}, nil
	}
	if headExpr == SPARSE {
		if len(*el) != 4 {
			return nil, fmt.Errorf("bad SPARSE expression format: %s", *el)
		}
		if _, sexp := resolveExpr((*el)[1]); sexp != nil {
			return nil, fmt.Errorf("bad SPARSE name: %s", *sexp)
		}
		shape, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, fmt.Errorf("bad SPARSE shape: %s, err: %s", (*el)[2].val, err)
		}
		// TODO(uuleon): hard coded dtype(double) should be removed
		return &featureSpec{
			FeatureName: (*el)[1].val,
			IsSparse:    false,
			Shape:       []int{shape},
			DType:       "double",
			Delimiter:   (*el)[3].val}, nil
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
	}

	switch headName {
	case DENSE:
		return resolveFeatureSpec(el)
	case SPARSE:
		return resolveFeatureSpec(el)
	case NUMERIC:
		if len(*el) != 3 {
			return nil, fmt.Errorf("bad NUMERIC expression format: %s", *el)
		}
		if _, sexp := resolveExpr((*el)[1]); sexp != nil {
			return nil, fmt.Errorf("bad NUMERIC key: %s", *sexp)
		}
		shape, err := strconv.Atoi((*el)[2].val)
		if err != nil {
			return nil, fmt.Errorf("bad NUMERIC shape: %s, err: %s", (*el)[2].val, err)
		}
		return &NumericColumn{
			Key:   (*el)[1].val,
			Shape: shape}, nil
	case BUCKET:
		if len(*el) != 3 {
			return nil, fmt.Errorf("bad BUCKET expression format: %s", *el)
		}
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
		b, err := transformToIntList(boundaries.([]interface{}))
		if err != nil {
			return nil, fmt.Errorf("bad BUCKET boundaries: %s", err)
		}
		return &BucketColumn{
			SourceColumn: *source.(*NumericColumn),
			Boundaries:   b}, nil
	case CROSS:
		if len(*el) != 3 {
			return nil, fmt.Errorf("bad CROSS expression format: %s", *el)
		}
		keysExpr := (*el)[1].sexp
		keys, err := resolveExprList(&keysExpr)
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
	case SQUARE:
		var list []interface{}
		for idx, expr := range *el {
			if idx > 0 {
				val, sexp := resolveExpr(expr)
				if sexp == nil {
					intVal, err := strconv.Atoi(val)
					if err != nil {
						list = append(list, val)
					} else {
						list = append(list, intVal)
					}
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
	return nil, nil
}

func transformToIntList(list []interface{}) ([]int, error) {
	var b = make([]int, len(list))
	for idx, item := range list {
		switch item.(type) {
		case int:
			b[idx] = item.(int)
		default:
			return nil, fmt.Errorf("type is not int: %s", item)
		}
	}
	return b, nil
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
