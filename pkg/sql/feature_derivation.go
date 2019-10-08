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
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"sqlflow.org/sqlflow/pkg/sql/codegen"
)

const featureDerivationRows = 1000

// FeatureColumnMap is like: target -> key -> FeatureColumn
type FeatureColumnMap map[string]map[string]codegen.FeatureColumn

// FieldMetaMap is a mapping from column name to ColumnSpec struct
type FieldMetaMap map[string]*codegen.FieldMeta

// makeFeatureColumnMap returns a map from column key to FeatureColumn
// NOTE that the target is not important for analyzing feature derivation.
func makeFeatureColumnMap(parsedFeatureColumns map[string][]codegen.FeatureColumn) FeatureColumnMap {
	fcMap := make(FeatureColumnMap)
	for target, fcList := range parsedFeatureColumns {
		fcMap[target] = make(map[string]codegen.FeatureColumn)
		for _, fc := range fcList {
			// CrossColumn use two columns as input, record the key for each column
			// FIXME(typhoonzero): how to handle duplicated keys?
			if cc, ok := fc.(*codegen.CrossColumn); ok {
				for _, k := range cc.Keys {
					if strKey, ok := k.(string); ok {
						fcMap[target][strKey] = &codegen.NumericColumn{
							FieldMeta: &codegen.FieldMeta{
								Name:      strKey,
								DType:     codegen.Float,
								Delimiter: "",
								Shape:     []int{1},
								IsSparse:  false,
							},
						}
					} else if fc, ok := k.(codegen.FeatureColumn); ok {
						fcMap[target][fc.GetFieldMeta().Name] = fc
					}
				}
			} else {
				// embedding column may got GetFieldMeta() == nil
				if emb, isEmb := fc.(*codegen.EmbeddingColumn); isEmb {
					if fc.GetFieldMeta() == nil {
						fcMap[target][emb.Name] = fc
					} else {
						fcMap[target][fc.GetFieldMeta().Name] = fc
					}
				} else {
					fcMap[target][fc.GetFieldMeta().Name] = fc
				}
			}
		}
	}
	return fcMap
}

// makeFieldMetaMap returns a map from column key to FieldMeta
// NOTE that the target is not important for analyzing feature derivation.
func makeFieldMetaMap(features map[string][]codegen.FeatureColumn) FieldMetaMap {
	// csMap := make(ColumnSpecMap)
	fmMap := make(FieldMetaMap)
	for _, fcList := range features {
		for _, fc := range fcList {
			if fc.GetFieldMeta() != nil {
				fmMap[fc.GetFieldMeta().Name] = fc.GetFieldMeta()
			}
		}
	}
	return fmMap
}

func newRowValue(columnTypeList []*sql.ColumnType) ([]interface{}, error) {
	rowData := make([]interface{}, len(columnTypeList))
	for idx, ct := range columnTypeList {
		typeName := ct.DatabaseTypeName()
		switch typeName {
		case "VARCHAR", "TEXT":
			rowData[idx] = new(string)
		case "INT":
			rowData[idx] = new(int32)
		case "BIGINT", "DECIMAL":
			rowData[idx] = new(int64)
		case "FLOAT":
			rowData[idx] = new(float32)
		case "DOUBLE":
			rowData[idx] = new(float64)
		default:
			return nil, fmt.Errorf("unsupported database column type: %s", typeName)
		}
	}
	return rowData, nil
}

func fillFieldMeta(columnTypeList []*sql.ColumnType, rowdata []interface{}, fieldMetaMap FieldMetaMap) error {
	csvRegex, err := regexp.Compile("(\\-?[0-9\\.]\\,)+(\\-?[0-9\\.])")
	if err != nil {
		return err
	}
	for idx, ct := range columnTypeList {
		_, fld := decomp(ct.Name())
		// add a default ColumnSpec for updating.
		if _, ok := fieldMetaMap[fld]; !ok {
			fieldMetaMap[fld] = &codegen.FieldMeta{
				Name:       fld,
				IsSparse:   false,
				Shape:      nil,
				DType:      codegen.Int,
				Delimiter:  "",
				Vocabulary: nil,
			}
		}
		// start the feature derivation routine
		typeName := ct.DatabaseTypeName()
		switch typeName {
		case "INT", "DECIMAL", "BIGINT":
			fieldMetaMap[fld].DType = codegen.Int
			fieldMetaMap[fld].Shape = []int{1}
		case "FLOAT", "DOUBLE":
			fieldMetaMap[fld].DType = codegen.Float
			fieldMetaMap[fld].Shape = []int{1}
		case "VARCHAR", "TEXT":
			cellData := rowdata[idx].(*string)
			if csvRegex.MatchString(*cellData) {
				// ----------------------- CSV string values -----------------------
				values := strings.Split(*cellData, ",")
				// set shape only when the column is "DENSE"
				if fieldMetaMap[fld].IsSparse == false && fieldMetaMap[fld].Shape == nil {
					fieldMetaMap[fld].Shape = []int{len(values)}
				}
				if fieldMetaMap[fld].IsSparse == false && fieldMetaMap[fld].Shape[0] != len(values) {
					return fmt.Errorf("column %s is csv format sparse tensor, but got DENSE column or not specified", fld)
				}
				fieldMetaMap[fld].Delimiter = ","
				// get dtype for csv values, use int64 and float32 only
				for _, v := range values {
					_, err := strconv.ParseInt(v, 10, 32)
					if err != nil {
						_, err := strconv.ParseFloat(v, 32)
						// set dtype to float32 once a float value come up
						if err == nil {
							fieldMetaMap[fld].DType = codegen.Float
						}
					}
				}
			} else {
				// -------------------- non-CSV string values --------------------
				_, err := strconv.ParseInt(*cellData, 10, 32)
				if err != nil {
					_, err := strconv.ParseFloat(*cellData, 32)
					if err == nil {
						// column is float value
						if fieldMetaMap[fld].Shape == nil {
							fieldMetaMap[fld].Shape = []int{1}
						}
						fieldMetaMap[fld].DType = codegen.Float
					} else {
						// neither int nor float, should deal with string dtype
						// to form a category_id_column
						fieldMetaMap[fld].DType = codegen.String
						if fieldMetaMap[fld].Vocabulary == nil {
							// initialize the vocabulary map
							fieldMetaMap[fld].Vocabulary = make(map[string]string)
						}
						if _, ok := fieldMetaMap[fld].Vocabulary[*cellData]; !ok {

							fieldMetaMap[fld].Vocabulary[*cellData] = *cellData
						}
					}
				} else {
					// column is int value
					if fieldMetaMap[fld].Shape == nil {
						fieldMetaMap[fld].Shape = []int{1}
					}
				}
			}
		default:
			return fmt.Errorf("unsupported database column type: %s", typeName)
		}
	}
	return nil
}

// InferFeatureColumns fill up featureColumn and columnSpec structs
// for all fields.
func InferFeatureColumns(ir *codegen.TrainIR) error {
	db, err := NewDB(ir.DataSource)
	if err != nil {
		return err
	}
	// Convert feature column list to a map
	fcMap := makeFeatureColumnMap(ir.Features)
	fmMap := makeFieldMetaMap(ir.Features)

	q := ir.Select
	re, err := regexp.Compile("LIMIT [0-9]+")
	if err != nil {
		return err
	}
	limitClauseIndexes := re.FindStringIndex(q)
	if limitClauseIndexes == nil {
		q = fmt.Sprintf("%s LIMIT %d", q, featureDerivationRows)
	} else {
		// TODO(typhoonzero): there may be complex SQL statements that contain multiple
		// LIMIT clause, using regex replace will replace them all.
		re.ReplaceAllString(q, fmt.Sprintf("LIMIT %d", featureDerivationRows))
	}

	rows, err := db.Query(q)
	if err != nil {
		return err
	}
	defer rows.Close()
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	selectFieldTypeMap := make(fieldTypes)
	selectFieldNames := []string{}
	for _, ct := range columnTypes {
		_, fld := decomp(ct.Name())
		typeName := ct.DatabaseTypeName()
		if _, ok := selectFieldTypeMap[fld]; ok {
			return fmt.Errorf("duplicated field name %s", fld)
		}
		selectFieldTypeMap[fld] = typeName
		selectFieldNames = append(selectFieldNames, fld)
	}

	for rows.Next() {
		rowData, err := newRowValue(columnTypes)
		if err != nil {
			return err
		}
		err = rows.Scan(rowData...)
		if err != nil {
			return err
		}
		fillFieldMeta(columnTypes, rowData, fmMap)
	}
	err = rows.Err()
	if err != nil {
		return err
	}

	// 1. Infer omited category_id_column for embedding_columns
	// 2. Add derivated feature column.
	//
	// need to store FeatureColumn under it's target in case of
	// the same column used for different target, e.g.
	// COLUMN EMBEDDING(c1) for deep
	//        EMBEDDING(c2) for deep
	//        EMBEDDING(c1) for wide
	for target := range ir.Features {
		for slctKey := range selectFieldTypeMap {
			fcTargetMap, ok := fcMap[target]
			if !ok {
				// create map for current target
				fcMap[target] = make(map[string]codegen.FeatureColumn)
				fcTargetMap = fcMap[target]
			}
			if fc, ok := fcTargetMap[slctKey]; ok {
				if embCol, isEmbCol := fc.(*codegen.EmbeddingColumn); isEmbCol {
					if embCol.CategoryColumn == nil {
						cs, ok := fmMap[embCol.Name]
						if !ok {
							return fmt.Errorf("column not found or infered: %s", embCol.Name)
						}
						// FIXME(typhoonzero): when to use sequence_category_id_column?
						embCol.CategoryColumn = &codegen.CategoryIDColumn{
							FieldMeta:  cs,
							BucketSize: cs.Shape[0],
						}
					}
				}
			} else {
				cs, ok := fmMap[slctKey]
				if !ok {
					return fmt.Errorf("column not found or infered: %s", slctKey)
				}
				if cs.DType != codegen.String {
					fcMap[target][slctKey] = &codegen.NumericColumn{
						FieldMeta: cs,
					}
				} else {
					// FIXME(typhoonzero): need full test case for string numeric columns
					fcMap[target][slctKey] = &codegen.CategoryIDColumn{
						FieldMeta:  cs,
						BucketSize: len(cs.Vocabulary),
					}
				}
			}
		}
	}
	// set back ir.Features in the order of select
	for target := range ir.Features {
		targetFeatureColumnMap := fcMap[target]
		ir.Features[target] = []codegen.FeatureColumn{}
		for _, slctKey := range selectFieldNames {
			// FIXME(typhoonzero): deal with cross column, do not add duplicate feature columns
			ir.Features[target] = append(ir.Features[target], targetFeatureColumnMap[slctKey])
		}
	}

	return nil
}
