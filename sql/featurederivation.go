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

	"github.com/sql-machine-learning/sqlflow/sql/columns"
)

const featureDerivationRows = 1000

type featureColumnMap map[string]columns.FeatureColumn
type columnSpecMap map[string]*columns.ColumnSpec

// makeFeatureColumnMap returns a map from column key to FeatureColumn
// NOTE that the target is not important for analyzing feature derivation.
func makeFeatureColumnMap(parsedFeatureColumns map[string][]columns.FeatureColumn) featureColumnMap {
	fcMap := make(featureColumnMap)
	for _, fcList := range parsedFeatureColumns {
		for _, fc := range fcList {
			fcMap[fc.GetKey()] = fc
		}
	}
	return fcMap
}

// makeColumnSpecMap returns a map from column key to ColumnSpec
// NOTE that the target is not important for analyzing feature derivation.
func makeColumnSpecMap(parsedColumnSpecs map[string][]*columns.ColumnSpec) columnSpecMap {
	csMap := make(columnSpecMap)
	for _, fcList := range parsedColumnSpecs {
		for _, cs := range fcList {
			csMap[cs.ColumnName] = cs
		}
	}
	return csMap
}

func newRowValue(columnTypeList []*sql.ColumnType) ([]interface{}, error) {
	rowData := make([]interface{}, len(columnTypeList))
	for idx, ct := range columnTypeList {
		typeName := ct.DatabaseTypeName()
		switch typeName {
		case "TEXT":
		case "VARCHAR":
			rowData[idx] = new(string)
		case "INT":
			rowData[idx] = new(int32)
		case "BIGINT":
		case "DECIMAL":
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

func fillColumnSpec(columnTypeList []*sql.ColumnType, rowdata []interface{}, csmap columnSpecMap) error {
	csvRegex, err := regexp.Compile("(\\-?[0-9\\.]\\,)+(\\-?[0-9\\.])")
	if err != nil {
		return err
	}
	for idx, ct := range columnTypeList {
		_, fld := decomp(ct.Name())
		// add a default ColumnSpec for updating.
		if _, ok := csmap[fld]; !ok {
			csmap[fld] = &columns.ColumnSpec{
				ColumnName: fld,
				IsSparse:   false,
				Shape:      nil,
				DType:      "int64",
				Delimiter:  "",
				Vocabulary: nil,
			}
		}
		// start the feature derivation routine
		typeName := ct.DatabaseTypeName()
		switch typeName {
		case "INT":
			csmap[fld].DType = "int32"
		case "BIGINT":
		case "DECIMAL":
			csmap[fld].DType = "int64"
		case "FLOAT":
			csmap[fld].DType = "float32"
		case "DOUBLE":
			csmap[fld].DType = "float64"
		case "TEXT":
		case "VARCHAR":
			cellData := rowdata[idx].(string)
			if csvRegex.MatchString(cellData) {
				// ----------------------- CSV string values -----------------------
				values := strings.Split(cellData, ",")
				// set shape only when the column is "DENSE"
				if csmap[fld].IsSparse == false && csmap[fld].Shape == nil {
					csmap[fld].Shape = []int{len(values)}
				}
				if csmap[fld].IsSparse == false && csmap[fld].Shape[0] != len(values) {
					return fmt.Errorf("column %s is csv format sparse tensor, but got DENSE column or not specified", fld)
				}
				csmap[fld].Delimiter = ","
				// get dtype for csv values, use int64 and float32 only
				for _, v := range values {
					_, err := strconv.ParseInt(v, 10, 32)
					if err != nil {
						_, err := strconv.ParseFloat(v, 32)
						// set dtype to float32 once a float value come up
						if err == nil {
							csmap[fld].DType = "float32"
						}
					}
				}
			} else {
				// -------------------- non-CSV string values --------------------
				_, err := strconv.ParseInt(cellData, 10, 32)
				if err != nil {
					_, err := strconv.ParseFloat(cellData, 32)
					if err == nil {
						// column is float value
						if csmap[fld].Shape == nil {
							csmap[fld].Shape = []int{1}
						}
						csmap[fld].DType = "float32"
					} else {
						// neither int nor float, should deal with string dtype
						// to form a category_id_column
						csmap[fld].DType = "string"
						if _, ok := csmap[fld].Vocabulary[cellData]; !ok {
							csmap[fld].Vocabulary[cellData] = cellData
						}
					}
				} else {
					// column is int value
					if csmap[fld].Shape == nil {
						csmap[fld].Shape = []int{1}
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
func InferFeatureColumns(slct *standardSelect,
	parsedFeatureColumns map[string][]columns.FeatureColumn,
	parsedColumnSpecs map[string][]*columns.ColumnSpec,
	connConfig *connectionConfig) error {
	// Convert feature column list to a map
	fcMap := makeFeatureColumnMap(parsedFeatureColumns)
	csMap := makeColumnSpecMap(parsedColumnSpecs)

	// TODO(typhoonzero): format connStr for hive/maxcompute
	connStr := fmt.Sprintf("%s://%s:%s@tcp(%s:%s)/", connConfig.Driver,
		connConfig.User, connConfig.Password,
		connConfig.Host, connConfig.Port)
	db, err := NewDB(connStr)
	if err != nil {
		return err
	}
	q := slct.String()
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

	log.Printf("feature derivation query: %s", q)
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
	for _, ct := range columnTypes {
		_, fld := decomp(ct.Name())
		typeName := ct.DatabaseTypeName()
		if _, ok := selectFieldTypeMap[fld]; ok {
			return fmt.Errorf("duplicated field name %s", fld)
		}
		selectFieldTypeMap[fld] = typeName
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
		fillColumnSpec(columnTypes, rowData, csMap)
	}
	err = rows.Err()
	if err != nil {
		return err
	}

	for slctKey, fieldType := range selectFieldTypeMap {
		// fill up FeatureColumn struct
		if fc, ok := fcMap[slctKey]; ok {
			if fc.GetColumnType() == columns.ColumnTypeEmbedding {
				fmt.Printf("automatically generate category_id_column here, fieldType: %v", fieldType)
			}
		}
	}

	return nil
}
