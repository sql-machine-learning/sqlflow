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

package ir

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/pipe"
	"sqlflow.org/sqlflow/go/verifier"
)

// ColumnMap is like: target -> key -> []FeatureColumn
// one column's data can be used by multiple feature columns, e.g.
// EMBEDDING(c1), CROSS(c1, c2)
type ColumnMap map[string]map[string][]FeatureColumn

// FieldDescMap is a mapping from column name to ColumnSpec struct
type FieldDescMap map[string]*FieldDesc

// makeColumnMap returns a map from column key to FeatureColumn
// NOTE that the target is not important for analyzing feature derivation.
func makeColumnMap(parsedFeatureColumns map[string][]FeatureColumn) ColumnMap {
	fcMap := make(ColumnMap)
	for target, fcList := range parsedFeatureColumns {
		fcMap[target] = make(map[string][]FeatureColumn)
		for _, fc := range fcList {
			initColumnMap(fcMap, fc, target)
		}
	}
	return fcMap
}

func initColumnMap(fcMap ColumnMap, fc FeatureColumn, target string) {
	switch c := fc.(type) {
	// embedding/indicator column may got len(GetFieldDesc()) == 0
	case *EmbeddingColumn:
		if len(fc.GetFieldDesc()) == 0 {
			fcMap[target][c.Name] = append(fcMap[target][c.Name], fc)
			return
		}

	case *IndicatorColumn:
		if len(fc.GetFieldDesc()) == 0 {
			fcMap[target][c.Name] = append(fcMap[target][c.Name], fc)
			return
		}
	}
	for _, fd := range fc.GetFieldDesc() {
		fcMap[target][fd.Name] = append(fcMap[target][fd.Name], fc)
	}
}

// makeFieldDescMap returns a map from column key to FieldDesc
// NOTE that the target is not important for analyzing feature derivation.
func makeFieldDescMap(features map[string][]FeatureColumn) FieldDescMap {
	fmMap := make(FieldDescMap)
	for _, fcList := range features {
		for _, fc := range fcList {
			for _, fm := range fc.GetFieldDesc() {
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

// scanRowValue returns the decoded row value from sql.Rows.
func scanRowValue(rows *sql.Rows, columnTypeList []*sql.ColumnType) ([]interface{}, error) {
	rowData := make([]interface{}, len(columnTypeList))
	for idx, ct := range columnTypeList {
		typeName := ct.DatabaseTypeName()
		switch unifyDatabaseTypeName(typeName) {
		case "CHAR", "VARCHAR", "TEXT", "STRING":
			rowData[idx] = new(string)
		case "INT", "TINYINT":
			rowData[idx] = new(int32)
		case "BIGINT", "DECIMAL":
			rowData[idx] = new(int64)
		case "FLOAT":
			rowData[idx] = new(float32)
		case "DOUBLE":
			rowData[idx] = new(float64)
		default:
			return nil, fmt.Errorf("scanRowValue: unsupported database column type: %s", typeName)
		}
	}
	if err := rows.Scan(rowData...); err != nil {
		return nil, err
	}
	return rowData, nil
}

func newDefaultFieldDesc(fieldName string) *FieldDesc {
	return &FieldDesc{
		Name:       fieldName,
		IsSparse:   false,
		Shape:      nil,
		DType:      Int,
		Delimiter:  "",
		Vocabulary: nil,
		MaxID:      0,
	}
}

// fillCSVFieldDesc will set fieldDescMap[fieldName] = FieldDesc for parsing the CSV data
func fillCSVFieldDesc(cellData string, fieldDescMap FieldDescMap, fieldName string) error {
	size := 1
	for _, s := range fieldDescMap[fieldName].Shape {
		size *= s
	}

	rawValues := strings.Split(cellData, ",")
	values := make([]string, 0, len(rawValues))
	for _, value := range rawValues {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			values = append(values, trimmedValue)
		}
	}

	// set shape only when the column is "DENSE"
	if fieldDescMap[fieldName].IsSparse == false && fieldDescMap[fieldName].Shape == nil {
		fieldDescMap[fieldName].Shape = []int{len(values)}
	}
	if fieldDescMap[fieldName].IsSparse == false && size != len(values) {
		if size > 1 {
			return fmt.Errorf("column %s should be csv format dense tensor of %d element(s), but got %d element(s)", fieldName, size, len(values))
		}
		// implicit set shape if shape is not provided in SQL COLUMN clause
		fieldDescMap[fieldName].Shape = []int{len(values)}
	}

	// FIXME(sneaxiy): currently, we only support sparse tensor in CSV format
	// whose values are 0 or 1. The numeric values in the cell data are the
	// indices where the values of the sparse tensor are 1. For example, the
	// cell value "3,5,7" indicates a sparse tensor x, and
	// x[3] = x[5] = x[7] = 1, and the other values of x are all zeros. Since
	// the index is always of integer type, we force to set the data type of
	// sparse tensor in CSV format is "Int". We should remove this constraint
	// if we will support other data formats in the future.
	if fieldDescMap[fieldName].IsSparse {
		fieldDescMap[fieldName].DType = Int
	}

	fieldDescMap[fieldName].Delimiter = ","
	// get dtype for csv values, use int64 and float32 only
	for _, v := range values {
		intValue, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			_, err := strconv.ParseFloat(v, 32)
			// set dtype to float32 once a float value come up
			if err == nil {
				fieldDescMap[fieldName].DType = Float
			}
		} else {
			// if the value is integer, record maxID
			if intValue > fieldDescMap[fieldName].MaxID {
				fieldDescMap[fieldName].MaxID = intValue
			}
		}
	}
	return nil
}

// fillFieldDescByDataType will set fieldDescMap[fieldName] = FieldDesc for parsing the numerical and string data
func fillFieldDescByDataType(cellData string, fieldDescMap FieldDescMap, fieldName string) {
	_, err := strconv.ParseInt(cellData, 10, 32)
	if err != nil {
		_, err := strconv.ParseFloat(cellData, 32)
		if err == nil {
			// column is float value
			if fieldDescMap[fieldName].Shape == nil {
				fieldDescMap[fieldName].Shape = []int{1}
			}
			fieldDescMap[fieldName].DType = Float
		} else {
			// neither int nor float, should deal with string dtype
			// to form a category_id_column
			fieldDescMap[fieldName].DType = String
			fieldDescMap[fieldName].Shape = []int{1}
			if fieldDescMap[fieldName].Vocabulary == nil {
				// initialize the vocabulary map
				fieldDescMap[fieldName].Vocabulary = make(map[string]string)
			}
			if _, ok := fieldDescMap[fieldName].Vocabulary[cellData]; !ok {
				fieldDescMap[fieldName].Vocabulary[cellData] = cellData
			}
		}
	} else {
		// column is int value
		if fieldDescMap[fieldName].Shape == nil {
			fieldDescMap[fieldName].Shape = []int{1}
		}
	}
}

const (
	csv = "csv"
	kv  = "kv"
)

func escapeDelimiter(delim string) string {
	if delim == "|" {
		return "\\|"
	} else if delim == "." {
		return "\\."
	} else if delim == "+" {
		return "\\+"
	} else if delim == "?" {
		return "\\?"
	} else if delim == "*" {
		return "\\*"
	} else if delim == "$" {
		return "\\$"
	} else if delim == " " {
		return "\\s"
	}
	return delim
}

func inferStringDataFormat(strData, delim1, delim2 string) string {
	const realNumberRegex = "((\\+|-)?([0-9]+)(\\.[0-9]+)?)|((\\+|-)?\\.?[0-9]+)"

	// string in the form of "3,5,7"
	csvRegex := regexp.MustCompile(fmt.Sprintf("^\\s*((%s)\\s*\\,\\s*)+(%s)\\s*(\\,?)\\s*$", realNumberRegex, realNumberRegex))
	if csvRegex.MatchString(strData) {
		return csv
	}

	// string in the form of "0:3.2 5:-0.5 7:9", default libsvm kv format have
	// delimi1==" ", delim2==":"
	keyValueRegex := regexp.MustCompile(fmt.Sprintf("^([0-9]+:(%s)\\s*)+$", realNumberRegex))
	if keyValueRegex.MatchString(strData) {
		return kv
	}
	if delim1 != "" && delim2 != "" {
		delim1 = escapeDelimiter(delim1)
		delim2 = escapeDelimiter(delim2)
		// string in the form of "k1:v1,k2:v2,k3:v3", where delim1==",", delim2==":"
		keyValueRegex = regexp.MustCompile(fmt.Sprintf("^((\\w|\\d)+(%s)?(%s)?(%s)?)+$", delim2, realNumberRegex, delim1))
		if keyValueRegex.MatchString(strData) {
			return kv
		}
	}
	return ""
}

func getMaxIndexOfKeyValueData(str string) (int, error) {
	maxIndex := 0
	// key-value string is like:
	// index:value index:value ...
	re := regexp.MustCompile("\\s+")
	for _, s := range re.Split(str, -1) {
		split := strings.SplitN(s, ":", 2)
		if len(split) != 2 {
			return 0, fmt.Errorf("invalid key-value format string %s", s)
		}

		index, err := strconv.Atoi(split[0])
		if err != nil {
			return 0, fmt.Errorf("invalid key-value format string %s", s)
		}

		if index > maxIndex {
			maxIndex = index
		}
	}
	return maxIndex, nil
}

func fillFieldDesc(columnTypeList []*sql.ColumnType, rowdata []interface{}, fieldDescMap FieldDescMap, rowCount int, originalSizes map[string]int) error {
	for idx, ct := range columnTypeList {
		_, fld := verifier.Decomp(ct.Name())
		// add a default ColumnSpec for updating.
		if _, ok := fieldDescMap[fld]; !ok {
			fieldDescMap[fld] = newDefaultFieldDesc(fld)
		}
		// start the feature derivation routine
		typeName := ct.DatabaseTypeName()
		switch unifyDatabaseTypeName(typeName) {
		case "INT", "TINYINT", "DECIMAL", "BIGINT":
			fieldDescMap[fld].DType = Int
			fieldDescMap[fld].Shape = []int{1}
		case "FLOAT", "DOUBLE":
			fieldDescMap[fld].DType = Float
			fieldDescMap[fld].Shape = []int{1}
		case "CHAR", "VARCHAR", "TEXT", "STRING":
			cellData := rowdata[idx].(*string)

			// Infer feature column type when rowCount == 0
			if rowCount == 0 {
				fieldDescMap[fld].Format = inferStringDataFormat(*cellData, fieldDescMap[fld].Delimiter, fieldDescMap[fld].DelimiterKV)
			}
			switch fieldDescMap[fld].Format {
			case csv:
				err := fillCSVFieldDesc(*cellData, fieldDescMap, fld)
				if err != nil {
					return err
				}
			case kv:
				if !fieldDescMap[fld].IsSparse {
					return fmt.Errorf(`should use "COLUMN SPARSE(%s)" for the key-value format data`, fld)
				}

				// fill FieldDesc for libsvm kv, general kv cell used for weighted
				// features need to set all attributes for SPARSE FieldDesc.
				if fieldDescMap[fld].DelimiterKV == "" {
					// TODO(sneaxiy): should we support int?
					fieldDescMap[fld].DType = Float
					// Only infer the dense shape when the original size is 1
					if size, ok := originalSizes[fld]; !ok || size == 1 {
						if rowCount == 0 {
							fieldDescMap[fld].Shape = []int{1}
						}

						curMaxIndex, err := getMaxIndexOfKeyValueData(*cellData)
						if err != nil {
							return err
						}
						if curMaxIndex+1 > fieldDescMap[fld].Shape[0] {
							fieldDescMap[fld].Shape[0] = curMaxIndex + 1
						}
					}
				}
			default:
				fillFieldDescByDataType(*cellData, fieldDescMap, fld)
			}
		default:
			return fmt.Errorf("fillFieldDesc: unsupported database column type: %s", typeName)
		}
	}
	return nil
}

// InferFeatureColumns fill up featureColumn and columnSpec structs
// for all fields.
// if wr is not nil, then write
func InferFeatureColumns(trainStmt *TrainStmt, db *database.DB) error {
	fcMap := makeColumnMap(trainStmt.Features)
	fmMap := makeFieldDescMap(trainStmt.Features)

	// TODO(typhoonzero): find a way to using subqueries like select * from (%s) AS a LIMIT 100
	// q := trainStmt.Select
	rows, err := verifier.FetchSamples(db, trainStmt.Select, 1000)
	if err != nil {
		return err
	}
	defer rows.Close()
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	selectFieldTypeMap := make(verifier.FieldTypes)
	selectFieldNames := []string{}
	for _, ct := range columnTypes {
		_, fld := verifier.Decomp(ct.Name())
		typeName := ct.DatabaseTypeName()
		if _, ok := selectFieldTypeMap[fld]; ok {
			return fmt.Errorf("duplicated field name %s", fld)
		}
		selectFieldTypeMap[fld] = typeName
		selectFieldNames = append(selectFieldNames, fld)
	}

	// get original size of each FieldDesc
	originalSizes := make(map[string]int, 0)
	for name, fc := range fmMap {
		size := 1
		for _, s := range fc.Shape {
			size *= s
		}
		originalSizes[name] = size
	}

	err = fillFieldDescs(rows, columnTypes, fmMap, originalSizes)
	if err != nil {
		return err
	}

	columnTargets := getFeatureColumnTargets(trainStmt)
	err = deriveFeatureColumn(fcMap, columnTargets, fmMap, selectFieldTypeMap, trainStmt)
	if err != nil {
		return err
	}
	// set back trainStmt.Features in the order of select and update trainStmt.Label
	setDerivedFeatureColumnToIR(trainStmt, fcMap, columnTargets, selectFieldNames)
	return deriveLabel(trainStmt, fmMap)
}

// getFeatureColumnTargets returns the list of strings, which will be used as
// the parameter keys when initialize a model, e.g.
// https://www.tensorflow.org/api_docs/python/tf/estimator/DNNLinearCombinedClassifier#__init__
// has parameters "linear_feature_columns", "dnn_feature_columns" accepts feature_columns.
func getFeatureColumnTargets(trainStmt *TrainStmt) []string {
	columnTargets := []string{}
	if len(trainStmt.Features) > 0 {
		for target := range trainStmt.Features {
			columnTargets = append(columnTargets, target)
		}
	} else {
		columnTargets = append(columnTargets, "feature_columns")
	}
	return columnTargets
}

// deriveFeatureColumn will fill in "fcMap" with derivated FeatureColumns.
func deriveFeatureColumn(fcMap ColumnMap, columnTargets []string, fdMap FieldDescMap, selectFieldTypeMap verifier.FieldTypes, trainStmt *TrainStmt) error {
	// 1. Infer omitted category_id_column for embedding_columns
	// 2. Add derivated feature column.
	//
	// need to store FeatureColumn under it's target in case of
	// the same column used for different target, e.g.
	// COLUMN EMBEDDING(c1) for deep
	//        EMBEDDING(c2) for deep
	//        EMBEDDING(c1) for wide
	for _, target := range columnTargets {
		fcTargetMap, ok := fcMap[target]
		if !ok {
			// create map for current target
			fcMap[target] = make(map[string][]FeatureColumn)
			fcTargetMap = fcMap[target]
		}
		fcMap[target] = make(map[string][]FeatureColumn)
		for f := range fcTargetMap {
			if _, ok := selectFieldTypeMap[f]; !ok {
				if len(fcTargetMap[f]) != 1 {
					return fmt.Errorf("cannot expand '%s' in 'column clause'", f)
				}
				// Try as regex to match the selected fields
				r, e := regexp.Compile("(?i)^" + f + "$")
				if e != nil {
					return fmt.Errorf("unknown column '%s' in 'column clause'", f)
				}
				hasMatch := false
				for sf := range selectFieldTypeMap {
					if r.MatchString(sf) {
						applied, err := fcTargetMap[f][0].ApplyTo(fdMap[sf])
						if err != nil {
							return err
						}
						fcMap[target][sf] = []FeatureColumn{applied}
						hasMatch = true
					}
				}
				if !hasMatch {
					return fmt.Errorf("'%s' in 'column clause' does not match any selected fields", f)
				}
				delete(fdMap, f)
			} else {
				fcMap[target][f] = fcTargetMap[f]
			}
		}
		fcTargetMap = fcMap[target]
		// ================== MAIN LOOP ==================
		// Update or generate FeatureColumn for each selected field:
		for slctKey := range selectFieldTypeMap {
			// skip label field
			if trainStmt.Label.GetFieldDesc()[0].Name == slctKey {
				continue
			}
			if fcList, ok := fcTargetMap[slctKey]; ok {
				err := updateFeatureColumn(fcList, fdMap)
				if err != nil {
					return err
				}
			} else {
				if len(columnTargets) > 1 {
					// if column clause have more than one target, each target should specify the
					// full list of the columns to use.
					continue
				}
				err := newFeatureColumn(fcTargetMap, fdMap, slctKey)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func fillFieldDescs(rows *sql.Rows, columnTypes []*sql.ColumnType, fmMap FieldDescMap, originalSizes map[string]int) error {
	rowCount := 0
	for rows.Next() {
		rowData, err := scanRowValue(rows, columnTypes)
		err = fillFieldDesc(columnTypes, rowData, fmMap, rowCount, originalSizes)
		if err != nil {
			return err
		}
		rowCount++
	}
	if rowCount == 0 && rows.Err() == nil {
		return fmt.Errorf("fillFieldDesc: empty dataset")
	}
	return rows.Err()
}

func updateFeatureColumn(fcList []FeatureColumn, fmMap FieldDescMap) error {
	for _, fc := range fcList {
		switch c := fc.(type) {
		case *EmbeddingColumn:
			if c.CategoryColumn == nil {
				cs, ok := fmMap[c.Name]
				if !ok {
					return fmt.Errorf("column not found or inferred: %s", c.Name)
				}
				// if column fieldDesc is SPARSE, the sparse shape should be in cs.Shape[0]
				bucketSize := int64(cs.Shape[0])
				// if the column is inferred as DENSE, use inferred MaxID as the
				// categoryIDColumns's bucket_size
				if cs.IsSparse == false {
					if cs.MaxID == 0 {
						return fmt.Errorf("use dense column on embedding column but did not got a correct MaxID")
					}
					bucketSize = cs.MaxID + 1
				}
				// FIXME(typhoonzero): when to use sequence_category_id_column?
				c.CategoryColumn = &CategoryIDColumn{
					FieldDesc:  cs,
					BucketSize: bucketSize,
				}
			}
		case *IndicatorColumn:
			if c.CategoryColumn == nil {
				cs, ok := fmMap[c.Name]
				if !ok {
					return fmt.Errorf("column not found or inferred: %s", c.Name)
				}
				// FIXME(typhoonzero): when to use sequence_category_id_column?
				// Use inferred MaxID as the categoryIDColumns's bucket_size
				if !cs.IsSparse {
					if cs.MaxID == 0 {
						return fmt.Errorf("use indicator column but did not got a correct MaxID")
					}
					c.CategoryColumn = &CategoryIDColumn{
						FieldDesc:  cs,
						BucketSize: cs.MaxID + 1,
					}
				} else {
					return fmt.Errorf("cannot use sparse column with indicator column")
				}
			}

		}
	}
	return nil
}

func newFeatureColumn(fcTargetMap map[string][]FeatureColumn, fmMap FieldDescMap, fieldName string) error {
	cs, ok := fmMap[fieldName]
	if !ok {
		return fmt.Errorf("column not found or inferred: %s", fieldName)
	}
	if cs.DType != String {
		fcTargetMap[fieldName] = append(fcTargetMap[fieldName],
			&NumericColumn{
				FieldDesc: cs,
			})
	} else {
		fcTargetMap[fieldName] = append(fcTargetMap[fieldName],
			&EmbeddingColumn{
				CategoryColumn: &CategoryIDColumn{
					FieldDesc:  cs,
					BucketSize: int64(len(cs.Vocabulary)),
				},
				// NOTE(typhoonzero): a default embedding size of 128 is enough for most cases.
				Dimension: 128,
				Combiner:  "sum",
			})
	}
	return nil
}

// setDerivedFeatureColumnToIR set derived feature column information back to the original IR structure.
func setDerivedFeatureColumnToIR(trainStmt *TrainStmt, fcMap ColumnMap, columnTargets []string, selectFieldNames []string) {
	for _, target := range columnTargets {
		// NOTE: some feature columns may contain more than 1 FieldDescs.
		// For example, "CROSS" may contain 2 or more FieldDescs. If there
		// is any feature column that contains more than 1 FieldDescs, they
		// would be placed at the end of TrainStmt.Features[target] according
		// to the order they appear in COLUMN clause. Therefore, we split the
		// feature columns to be 2 slices: singleColumnFC and multiColumnFC.
		// In order to know the order each feature column appears in COLUMN
		// clause, we should collect all feature columns that contain more than
		// 1 FieldDescs beforehand, i.e., allMultiColumnFeatures, so that we
		// can sort multiColumnFC afterwards.
		allMultiColumnFeatures := make([]FeatureColumn, 0)
		for _, fc := range trainStmt.Features[target] {
			if len(fc.GetFieldDesc()) > 1 {
				allMultiColumnFeatures = append(allMultiColumnFeatures, fc)
			}
		}

		targetFeatureColumnMap := fcMap[target]
		singleColumnFC := make([]FeatureColumn, 0)
		multiColumnFC := make([]FeatureColumn, 0)
		allColumnFCs := make([]FeatureColumn, 0)
		for _, slctKey := range selectFieldNames {
			// label should not be added to feature columns
			if slctKey == trainStmt.Label.GetFieldDesc()[0].Name {
				continue
			}

			for _, fc := range targetFeatureColumnMap[slctKey] {
				exists := false
				for _, resultFC := range allColumnFCs {
					if fc == resultFC {
						exists = true
						break
					}
				}

				if !exists {
					allColumnFCs = append(allColumnFCs, fc)
					if len(fc.GetFieldDesc()) == 1 {
						singleColumnFC = append(singleColumnFC, fc)
					} else {
						multiColumnFC = append(multiColumnFC, fc)
					}
				}
			}
		}

		if len(multiColumnFC) > 0 {
			indices := make([]int, 0)
			for _, fcRet := range multiColumnFC {
				for j, v := range allMultiColumnFeatures {
					if fcRet == v {
						indices = append(indices, j)
						break
					}
				}
			}

			// sort multiColumnFC according to the order they appear in allMultiColumnFeatures,
			// i.e., the order they appear in COLUMN clause.
			sort.Slice(multiColumnFC, func(i, j int) bool { return indices[i] < indices[j] })
			allColumnFCs = append(singleColumnFC, multiColumnFC...)
		}

		trainStmt.Features[target] = allColumnFCs
	}
}

// deriveLabel set derived label FieldDesc information back to the original IR structure.
func deriveLabel(trainStmt *TrainStmt, fmMap FieldDescMap) error {
	labelName := trainStmt.Label.GetFieldDesc()[0].Name
	if labelName == "" {
		return nil // NOTE: clustering model may not specify Label
	}
	if fmMap[labelName] == nil {
		return fmt.Errorf("deriveLabel: LABEL COLUMN '%s' not found", labelName)
	}
	trainStmt.Label = &NumericColumn{
		FieldDesc: fmMap[labelName],
	}
	// use shape [] if label shape is [1] for TensorFlow scalar label shape should be [].
	shape := trainStmt.Label.GetFieldDesc()[0].Shape
	if len(shape) == 1 && shape[0] == 1 {
		trainStmt.Label.GetFieldDesc()[0].Shape = []int{}
	}
	return nil
}

// LogDerivationResult write messages to wr to log the feature derivation results
func LogDerivationResult(wr *pipe.Writer, trainStmt *TrainStmt) {
	if wr != nil {
		for target, fclist := range trainStmt.Features {
			for _, fc := range fclist {
				for _, fm := range fc.GetFieldDesc() {
					wr.Write(fmt.Sprintf("Using column (%s) in feature column (%T) as model construct param (%s)", fm.Name, fc, target))
				}
			}
		}
		wr.Write("\n")
	}
}
