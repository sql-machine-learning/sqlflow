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

	"sqlflow.org/sqlflow/pkg/sql/ir"
)

const featureDerivationRows = 1000

// FeatureColumnMap is like: target -> key -> []FeatureColumn
// one column's data can be used by multiple feature columns, e.g.
// EMBEDDING(c1), CROSS(c1, c2)
type FeatureColumnMap map[string]map[string][]ir.FeatureColumn

// FieldMetaMap is a mapping from column name to ColumnSpec struct
type FieldMetaMap map[string]*ir.FieldMeta

// makeFeatureColumnMap returns a map from column key to FeatureColumn
// NOTE that the target is not important for analyzing feature derivation.
func makeFeatureColumnMap(parsedFeatureColumns map[string][]ir.FeatureColumn) FeatureColumnMap {
	fcMap := make(FeatureColumnMap)
	for target, fcList := range parsedFeatureColumns {
		fcMap[target] = make(map[string][]ir.FeatureColumn)
		for _, fc := range fcList {
			// CrossColumn use two columns as input, record the key for each column
			if cc, ok := fc.(*ir.CrossColumn); ok {
				for idx, k := range cc.Keys {
					// if the key of CrossColumn is a string, generate a default numeric column.
					if strKey, ok := k.(string); ok {
						cc.Keys[idx] = &ir.NumericColumn{
							FieldMeta: &ir.FieldMeta{
								Name:      strKey,
								DType:     ir.Float,
								Delimiter: "",
								Shape:     []int{1},
								IsSparse:  false,
							},
						}
						fcMap[target][strKey] = append(fcMap[target][strKey], cc)
					} else if nc, ok := k.(*ir.NumericColumn); ok {
						fcMap[target][nc.FieldMeta.Name] = append(fcMap[target][nc.FieldMeta.Name], cc)
					}
				}
			} else {
				// embedding column may got len(GetFieldMeta()) == 0
				if emb, isEmb := fc.(*ir.EmbeddingColumn); isEmb {
					if len(fc.GetFieldMeta()) == 0 {
						fcMap[target][emb.Name] = append(fcMap[target][emb.Name], fc)
					} else {
						fcMap[target][fc.GetFieldMeta()[0].Name] = append(fcMap[target][fc.GetFieldMeta()[0].Name], fc)
					}
				} else {
					fcMap[target][fc.GetFieldMeta()[0].Name] = append(fcMap[target][fc.GetFieldMeta()[0].Name], fc)
				}
			}
		}
	}
	return fcMap
}

// makeFieldMetaMap returns a map from column key to FieldMeta
// NOTE that the target is not important for analyzing feature derivation.
func makeFieldMetaMap(features map[string][]ir.FeatureColumn) FieldMetaMap {
	fmMap := make(FieldMetaMap)
	for _, fcList := range features {
		for _, fc := range fcList {
			for _, fm := range fc.GetFieldMeta() {
				if fm != nil {
					fmMap[fm.Name] = fm
				}
			}
		}
	}
	return fmMap
}

func unifyDatabaseTypeName(typeName string) string {
	// NOTE(typhoonzero): Hive uses typenames like "XXX_TYPE"
	if strings.HasSuffix(typeName, "_TYPE") {
		typeName = strings.Replace(typeName, "_TYPE", "", 1)
	}

	// NOTE(tony): MaxCompute type name is in lower cases
	return strings.ToUpper(typeName)
}

func newRowValue(columnTypeList []*sql.ColumnType) ([]interface{}, error) {
	rowData := make([]interface{}, len(columnTypeList))
	for idx, ct := range columnTypeList {
		typeName := ct.DatabaseTypeName()
		switch unifyDatabaseTypeName(typeName) {
		case "VARCHAR", "TEXT", "STRING":
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
			return nil, fmt.Errorf("newRowValue: unsupported database column type: %s", typeName)
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
			fieldMetaMap[fld] = &ir.FieldMeta{
				Name:       fld,
				IsSparse:   false,
				Shape:      nil,
				DType:      ir.Int,
				Delimiter:  "",
				Vocabulary: nil,
				MaxID:      0,
			}
		}
		// start the feature derivation routine
		typeName := ct.DatabaseTypeName()
		switch unifyDatabaseTypeName(typeName) {
		case "INT", "DECIMAL", "BIGINT":
			fieldMetaMap[fld].DType = ir.Int
			fieldMetaMap[fld].Shape = []int{1}
		case "FLOAT", "DOUBLE":
			fieldMetaMap[fld].DType = ir.Float
			fieldMetaMap[fld].Shape = []int{1}
		case "VARCHAR", "TEXT", "STRING":
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
					intValue, err := strconv.ParseInt(v, 10, 64)
					if err != nil {
						_, err := strconv.ParseFloat(v, 32)
						// set dtype to float32 once a float value come up
						if err == nil {
							fieldMetaMap[fld].DType = ir.Float
						}
					} else {
						// if the value is integer, record maxID
						if intValue > fieldMetaMap[fld].MaxID {
							fieldMetaMap[fld].MaxID = intValue
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
						fieldMetaMap[fld].DType = ir.Float
					} else {
						// neither int nor float, should deal with string dtype
						// to form a category_id_column
						fieldMetaMap[fld].DType = ir.String
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
			return fmt.Errorf("fillFieldMeta: unsupported database column type: %s", typeName)
		}
	}
	return nil
}

// InferFeatureColumns fill up featureColumn and columnSpec structs
// for all fields.
func InferFeatureColumns(trainIR *ir.TrainClause) error {
	db, err := NewDB(trainIR.DataSource)
	if err != nil {
		return err
	}
	// Convert feature column list to a map
	fcMap := makeFeatureColumnMap(trainIR.Features)
	fmMap := makeFieldMetaMap(trainIR.Features)

	// TODO(typhoonzero): find a way to using subqueries like select * from (%s) AS a LIMIT 100
	q := trainIR.Select
	re, err := regexp.Compile("(LIMIT [0-9]+|limit [0-9]+)")
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
		err = fillFieldMeta(columnTypes, rowData, fmMap)
		if err != nil {
			return err
		}
	}
	err = rows.Err()
	if err != nil {
		return err
	}

	// 1. Infer omitted category_id_column for embedding_columns
	// 2. Add derivated feature column.
	//
	// need to store FeatureColumn under it's target in case of
	// the same column used for different target, e.g.
	// COLUMN EMBEDDING(c1) for deep
	//        EMBEDDING(c2) for deep
	//        EMBEDDING(c1) for wide
	columnTargets := []string{}
	if len(trainIR.Features) > 0 {
		for target := range trainIR.Features {
			columnTargets = append(columnTargets, target)
		}
	} else {
		columnTargets = append(columnTargets, "feature_columns")
	}
	for _, target := range columnTargets {
		for slctKey := range selectFieldTypeMap {
			if slctKey == trainIR.Label.GetFieldMeta()[0].Name {
				// skip label field
				continue
			}
			fcTargetMap, ok := fcMap[target]
			if !ok {
				// create map for current target
				fcMap[target] = make(map[string][]ir.FeatureColumn)
				fcTargetMap = fcMap[target]
			}
			if fcList, ok := fcTargetMap[slctKey]; ok {
				for _, fc := range fcList {
					if embCol, isEmbCol := fc.(*ir.EmbeddingColumn); isEmbCol {
						if embCol.CategoryColumn == nil {
							cs, ok := fmMap[embCol.Name]
							if !ok {
								return fmt.Errorf("column not found or inferred: %s", embCol.Name)
							}
							// FIXME(typhoonzero): when to use sequence_category_id_column?
							// if column fieldMeta is SPARSE, the sparse shape should be in cs.Shape[0]
							bucketSize := int64(cs.Shape[0])
							// if the column is inferred as DENSE, use inferred MaxID as the
							// categoryIDColumns's bucket_size
							if cs.IsSparse == false {
								if cs.MaxID == 0 {
									return fmt.Errorf("use dense column on embedding column but did not got a correct MaxID")
								}
								bucketSize = cs.MaxID + 1
							}
							embCol.CategoryColumn = &ir.CategoryIDColumn{
								FieldMeta:  cs,
								BucketSize: bucketSize,
							}
						}
					}
				}
			} else {
				if len(columnTargets) > 1 {
					// if column clause have more than one target, each target should specify the
					// full list of the columns to use.
					continue
				}
				cs, ok := fmMap[slctKey]
				if !ok {
					return fmt.Errorf("column not found or inferred: %s", slctKey)
				}
				if cs.DType != ir.String {
					fcMap[target][slctKey] = append(fcMap[target][slctKey],
						&ir.NumericColumn{
							FieldMeta: cs,
						})
				} else {
					// FIXME(typhoonzero): need full test case for string numeric columns
					fcMap[target][slctKey] = append(fcMap[target][slctKey],
						&ir.CategoryIDColumn{
							FieldMeta:  cs,
							BucketSize: int64(len(cs.Vocabulary)),
						})
				}
			}
		}
	}

	// set back trainIR.Features in the order of select
	for _, target := range columnTargets {
		targetFeatureColumnMap := fcMap[target]
		trainIR.Features[target] = []ir.FeatureColumn{}
		// append cross columns at the end of all selected fields.
		crossColumns := []*ir.CrossColumn{}
		for _, slctKey := range selectFieldNames {
			// label should not be added to feature columns
			if slctKey == trainIR.Label.GetFieldMeta()[0].Name {
				continue
			}
			for _, fc := range targetFeatureColumnMap[slctKey] {
				if cc, ok := fc.(*ir.CrossColumn); ok {
					crossColumns = append(crossColumns, cc)
					continue
				}
				trainIR.Features[target] = append(trainIR.Features[target], fc)
			}
		}
		// remove duplicated CrossColumns pointers, for CROSS(c1, c2), both fcMap[c1], fcMap[c2] will
		// record the CrossColumn pointer
		for i := 0; i < len(crossColumns); i++ {
			exists := false
			for v := 0; v < i; v++ {
				if crossColumns[v] == crossColumns[i] {
					exists = true
					break
				}
			}
			if !exists {
				trainIR.Features[target] = append(trainIR.Features[target], crossColumns[i])
			}
		}
	}
	return nil
}
