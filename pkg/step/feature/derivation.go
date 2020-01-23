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

package feature

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"sqlflow.org/sqlflow/pkg/database"
	"sqlflow.org/sqlflow/pkg/ir"
	"sqlflow.org/sqlflow/pkg/pipe"
)

const featureDerivationRows = 1000

// TODO(typhoonzero): fieldTypes are copied from verifier.go, need refactor.
type fieldTypes map[string]string

// TODO(typhoonzero): decomp is copied from verifier.go, need to refactor the structure to avoid copying.
func decomp(ident string) (tbl string, fld string) {
	// Note: Hive driver represents field names in lower cases, so we convert all identifier
	// to lower case
	ident = strings.ToLower(ident)
	idx := strings.LastIndex(ident, ".")
	if idx == -1 {
		return "", ident
	}
	return ident[0:idx], ident[idx+1:]
}

// ColumnMap is like: target -> key -> []FeatureColumn
// one column's data can be used by multiple feature columns, e.g.
// EMBEDDING(c1), CROSS(c1, c2)
type ColumnMap map[string]map[string][]ir.FeatureColumn

// FieldDescMap is a mapping from column name to ColumnSpec struct
type FieldDescMap map[string]*ir.FieldDesc

// makeColumnMap returns a map from column key to FeatureColumn
// NOTE that the target is not important for analyzing feature derivation.
func makeColumnMap(parsedFeatureColumns map[string][]ir.FeatureColumn) ColumnMap {
	fcMap := make(ColumnMap)
	for target, fcList := range parsedFeatureColumns {
		fcMap[target] = make(map[string][]ir.FeatureColumn)
		for _, fc := range fcList {
			initColumnMap(fcMap, fc, target)
		}
	}
	return fcMap
}

func initColumnMap(fcMap ColumnMap, fc ir.FeatureColumn, target string) {
	// CrossColumn use two columns as input, record the key for each column
	if cc, ok := fc.(*ir.CrossColumn); ok {
		for idx, k := range cc.Keys {
			// if the key of CrossColumn is a string, generate a default numeric column.
			if strKey, ok := k.(string); ok {
				cc.Keys[idx] = &ir.NumericColumn{
					FieldDesc: &ir.FieldDesc{
						Name:      strKey,
						DType:     ir.Float,
						Delimiter: "",
						Shape:     []int{1},
						IsSparse:  false,
					},
				}
				fcMap[target][strKey] = append(fcMap[target][strKey], cc)
			} else if nc, ok := k.(*ir.NumericColumn); ok {
				fcMap[target][nc.FieldDesc.Name] = append(fcMap[target][nc.FieldDesc.Name], cc)
			}
		}
	} else {
		// embedding column may got len(GetFieldDesc()) == 0
		if emb, isEmb := fc.(*ir.EmbeddingColumn); isEmb {
			if len(fc.GetFieldDesc()) == 0 {
				fcMap[target][emb.Name] = append(fcMap[target][emb.Name], fc)
			} else {
				fcMap[target][fc.GetFieldDesc()[0].Name] = append(fcMap[target][fc.GetFieldDesc()[0].Name], fc)
			}
		} else {
			fcMap[target][fc.GetFieldDesc()[0].Name] = append(fcMap[target][fc.GetFieldDesc()[0].Name], fc)
		}
	}
}

// makeFieldDescMap returns a map from column key to FieldDesc
// NOTE that the target is not important for analyzing feature derivation.
func makeFieldDescMap(features map[string][]ir.FeatureColumn) FieldDescMap {
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

func newDefaultFieldDesc(fieldName string) *ir.FieldDesc {
	return &ir.FieldDesc{
		Name:       fieldName,
		IsSparse:   false,
		Shape:      nil,
		DType:      ir.Int,
		Delimiter:  "",
		Vocabulary: nil,
		MaxID:      0,
	}
}

// fillCSVFieldDesc will set fieldDescMap[fieldName] = FieldDesc for parsing the CSV data
func fillCSVFieldDesc(cellData string, fieldDescMap FieldDescMap, fieldName string) error {
	values := strings.Split(cellData, ",")
	// set shape only when the column is "DENSE"
	if fieldDescMap[fieldName].IsSparse == false && fieldDescMap[fieldName].Shape == nil {
		fieldDescMap[fieldName].Shape = []int{len(values)}
	}
	if fieldDescMap[fieldName].IsSparse == false && fieldDescMap[fieldName].Shape[0] != len(values) {
		return fmt.Errorf("column %s is csv format sparse tensor, but got DENSE column or not specified", fieldName)
	}
	fieldDescMap[fieldName].Delimiter = ","
	// get dtype for csv values, use int64 and float32 only
	for _, v := range values {
		intValue, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			_, err := strconv.ParseFloat(v, 32)
			// set dtype to float32 once a float value come up
			if err == nil {
				fieldDescMap[fieldName].DType = ir.Float
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

// fillNonCSVFieldDesc will set fieldDescMap[fieldName] = FieldDesc for parsing the numerical and string data
func fillNonCSVFieldDesc(cellData string, fieldDescMap FieldDescMap, fieldName string) error {
	_, err := strconv.ParseInt(cellData, 10, 32)
	if err != nil {
		_, err := strconv.ParseFloat(cellData, 32)
		if err == nil {
			// column is float value
			if fieldDescMap[fieldName].Shape == nil {
				fieldDescMap[fieldName].Shape = []int{1}
			}
			fieldDescMap[fieldName].DType = ir.Float
		} else {
			// neither int nor float, should deal with string dtype
			// to form a category_id_column
			fieldDescMap[fieldName].DType = ir.String
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
	return nil
}

func fillFieldDesc(columnTypeList []*sql.ColumnType, rowdata []interface{}, fieldDescMap FieldDescMap) error {
	csvRegex, err := regexp.Compile("(\\-?[0-9\\.]\\,)+(\\-?[0-9\\.])")
	if err != nil {
		return err
	}
	for idx, ct := range columnTypeList {
		_, fld := decomp(ct.Name())
		// add a default ColumnSpec for updating.
		if _, ok := fieldDescMap[fld]; !ok {
			fieldDescMap[fld] = newDefaultFieldDesc(fld)
		}
		// start the feature derivation routine
		typeName := ct.DatabaseTypeName()
		switch unifyDatabaseTypeName(typeName) {
		case "INT", "DECIMAL", "BIGINT":
			fieldDescMap[fld].DType = ir.Int
			fieldDescMap[fld].Shape = []int{1}
		case "FLOAT", "DOUBLE":
			fieldDescMap[fld].DType = ir.Float
			fieldDescMap[fld].Shape = []int{1}
		case "VARCHAR", "TEXT", "STRING":
			cellData := rowdata[idx].(*string)
			if csvRegex.MatchString(*cellData) {
				fillCSVFieldDesc(*cellData, fieldDescMap, fld)
			} else {
				fillNonCSVFieldDesc(*cellData, fieldDescMap, fld)
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
func InferFeatureColumns(trainStmt *ir.TrainStmt, dataSource string) error {
	fcMap := makeColumnMap(trainStmt.Features)
	fmMap := makeFieldDescMap(trainStmt.Features)

	// TODO(typhoonzero): find a way to using subqueries like select * from (%s) AS a LIMIT 100
	// q := trainStmt.Select
	rows, err := fetchSamples(dataSource, trainStmt.Select)
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

	err = fillFieldDescs(rows, columnTypes, fmMap)
	if err != nil {
		return err
	}

	columnTargets := getFeatureColumnTargets(trainStmt)
	err = deriveFeatureColumn(fcMap, columnTargets, fmMap, selectFieldTypeMap, trainStmt)
	// set back trainStmt.Features in the order of select and update trainStmt.Label
	setBackToIR(trainStmt, fcMap, columnTargets, selectFieldNames)
	setBackToLabel(trainStmt, fmMap)
	return nil
}

// getFeatureColumnTargets returns the list of strings, which will be used as
// the parameter keys when initialize a model, e.g.
// https://www.tensorflow.org/api_docs/python/tf/estimator/DNNLinearCombinedClassifier#__init__
// has parameters "linear_feature_columns", "dnn_feature_columns" accepts feature_columns.
func getFeatureColumnTargets(trainStmt *ir.TrainStmt) []string {
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
func deriveFeatureColumn(fcMap ColumnMap, columnTargets []string, fmMap FieldDescMap, selectFieldTypeMap fieldTypes, trainStmt *ir.TrainStmt) error {
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
			fcMap[target] = make(map[string][]ir.FeatureColumn)
			fcTargetMap = fcMap[target]
		}
		// ================== MAIN LOOP ==================
		// Update or generate FeatureColumn for each selected field:
		for slctKey := range selectFieldTypeMap {
			// skip label field
			if trainStmt.Label.GetFieldDesc()[0].Name == slctKey {
				continue
			}
			if fcList, ok := fcTargetMap[slctKey]; ok {
				err := updateFeatureColumn(fcList, fmMap)
				if err != nil {
					return err
				}
			} else {
				if len(columnTargets) > 1 {
					// if column clause have more than one target, each target should specify the
					// full list of the columns to use.
					continue
				}
				err := newFeatureColumn(fcTargetMap, fmMap, slctKey)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func fetchSamples(dataSource string, query string) (*sql.Rows, error) {
	db, err := database.OpenAndConnectDB(dataSource)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile("(?i)LIMIT [0-9]+")
	if err != nil {
		return nil, err
	}
	limitClauseIndexes := re.FindStringIndex(query)
	if limitClauseIndexes == nil {
		query = fmt.Sprintf("%s LIMIT %d", query, featureDerivationRows)
	} else {
		// TODO(typhoonzero): there may be complex SQL statements that contain multiple
		// LIMIT clause, using regex replace will replace them all.
		re.ReplaceAllString(query, fmt.Sprintf("LIMIT %d", featureDerivationRows))
	}
	return db.Query(query)
}

func fillFieldDescs(rows *sql.Rows, columnTypes []*sql.ColumnType, fmMap FieldDescMap) error {
	for rows.Next() {
		rowData, err := newRowValue(columnTypes)
		if err != nil {
			return err
		}
		err = rows.Scan(rowData...)
		if err != nil {
			return err
		}
		err = fillFieldDesc(columnTypes, rowData, fmMap)
		if err != nil {
			return err
		}
	}
	return rows.Err()
}

func updateFeatureColumn(fcList []ir.FeatureColumn, fmMap FieldDescMap) error {
	for _, fc := range fcList {
		if embCol, isEmbCol := fc.(*ir.EmbeddingColumn); isEmbCol {
			if embCol.CategoryColumn == nil {
				cs, ok := fmMap[embCol.Name]
				if !ok {
					return fmt.Errorf("column not found or inferred: %s", embCol.Name)
				}
				// FIXME(typhoonzero): when to use sequence_category_id_column?
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
				embCol.CategoryColumn = &ir.CategoryIDColumn{
					FieldDesc:  cs,
					BucketSize: bucketSize,
				}
			}
		}
	}
	return nil
}

func newFeatureColumn(fcTargetMap map[string][]ir.FeatureColumn, fmMap FieldDescMap, fieldName string) error {
	cs, ok := fmMap[fieldName]
	if !ok {
		return fmt.Errorf("column not found or inferred: %s", fieldName)
	}
	if cs.DType != ir.String {
		fcTargetMap[fieldName] = append(fcTargetMap[fieldName],
			&ir.NumericColumn{
				FieldDesc: cs,
			})
	} else {
		fcTargetMap[fieldName] = append(fcTargetMap[fieldName],
			&ir.EmbeddingColumn{
				CategoryColumn: &ir.CategoryIDColumn{
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

// setBackToIR set derived feature column information back to the original IR structure.
func setBackToIR(trainStmt *ir.TrainStmt, fcMap ColumnMap, columnTargets []string, selectFieldNames []string) {
	for _, target := range columnTargets {
		targetFeatureColumnMap := fcMap[target]
		trainStmt.Features[target] = []ir.FeatureColumn{}
		// append cross columns at the end of all selected fields.
		crossColumns := []*ir.CrossColumn{}
		for _, slctKey := range selectFieldNames {
			// label should not be added to feature columns
			if slctKey == trainStmt.Label.GetFieldDesc()[0].Name {
				continue
			}
			for _, fc := range targetFeatureColumnMap[slctKey] {
				if cc, ok := fc.(*ir.CrossColumn); ok {
					crossColumns = append(crossColumns, cc)
					continue
				}
				trainStmt.Features[target] = append(trainStmt.Features[target], fc)
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
				trainStmt.Features[target] = append(trainStmt.Features[target], crossColumns[i])
			}
		}
	}
}

// setBackToLabel set derived label FieldDesc information back to the original IR structure.
func setBackToLabel(trainStmt *ir.TrainStmt, fmMap FieldDescMap) {
	labelName := trainStmt.Label.GetFieldDesc()[0].Name
	if labelName == "" {
		return // NOTE: clustering model may not specify Label
	}
	trainStmt.Label = &ir.NumericColumn{
		FieldDesc: fmMap[labelName],
	}
	// use shape [] if label shape is [1] for Tensorflow scalar label shape should be [].
	shape := trainStmt.Label.GetFieldDesc()[0].Shape
	if len(shape) == 1 && shape[0] == 1 {
		trainStmt.Label.GetFieldDesc()[0].Shape = []int{}
	}
}

// LogDerivationResult write messages to wr to log the feature derivation results
func LogDerivationResult(wr *pipe.Writer, trainStmt *ir.TrainStmt) {
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
