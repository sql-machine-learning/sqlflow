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

	"github.com/pkg/errors"
)

type trainAndValDataset struct {
	// `supported` here identifies if SQLFlow is able to split dataset into
	// training dataset and validation dataset.
	// So, TODO(weiguo): Let's remove `supproted` if SQLFlow supports other
	// drivers, like: MySQL, hive(specified in database.go:open()).
	supported  bool
	table      string
	training   string // table for training: < k
	validation string // table for validation: >= k
}

const (
	temporaryTableLifecycle = 14 // day(s)
	randomColumn            = "sqlflow_rdm"
	tablePrefix             = "sqlflow_tv" // 'tv' = training & validation
	trainingPrefix          = "sqlflow_training"
	validationPrefix        = "sqlflow_validation"
	suffix                  = "sqlflow"
)

var (
	errBadBoundary = errors.New("boundary should between (0.0, 1.0) exclude")
)

// newTrainAndValDataset generates a temporary table, + sqlflow_randowm column
func newTrainAndValDataset(db *DB, slct string, origTable string, trainingUpperbound float32) (*trainAndValDataset, error) {
	if trainingUpperbound <= 0 || trainingUpperbound >= 1 {
		return nil, errBadBoundary
	}

	switch db.driverName {
	case "maxcompute":
		return createMaxcomputeDataset(db, slct, origTable, trainingUpperbound)
		// TODO(weiguo): support other databases, like: "hive", "mysql"...
	default:
		return nil, nil
	}
}

func releaseTrainAndValDataset(ds *trainAndValDataset) {
	// TODO(weiguo): release resources for databases, like: "hive", "mysql"...
}

func createMaxcomputeDataset(db *DB, slct string, origTable string, trainingUpperbound float32) (*trainAndValDataset, error) {
	ds := namingTrainAndValDataset(origTable)
	// drop the table if already exist
	dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s;", ds.table)
	if _, e := db.Exec(dropStmt); e != nil {
		log.Errorf("drop temporary table failed, stmt:[%s], err:%v", dropStmt, e)
		return nil, e
	}
	// create a table, then split it into train and val tables
	stmt := fmt.Sprintf("CREATE TABLE %s LIFECYCLE %d AS SELECT *, RAND() AS %s FROM (%s) AS %s_ori", ds.table, temporaryTableLifecycle, randomColumn, slct, ds.table)
	if _, e := db.Exec(stmt); e != nil {
		log.Errorf("create temporary table failed, stmt:[%s], err:%v", stmt, e)
		return nil, e
	}
	trainingCond := fmt.Sprintf("%s < %f", randomColumn, trainingUpperbound)
	if e := createMaxcomputeTable(ds.training, ds.table, db, trainingCond); e != nil {
		return nil, e
	}
	validationCond := fmt.Sprintf("%s >= %f", randomColumn, trainingUpperbound)
	if e := createMaxcomputeTable(ds.validation, ds.table, db, validationCond); e != nil {
		return nil, e
	}
	return ds, nil
}

func createMaxcomputeTable(target, origin string, db *DB, cond string) error {
	// drop the table if already exist
	dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s;", target)
	if _, e := db.Exec(dropStmt); e != nil {
		log.Errorf("drop temporary table failed, stmt:[%s], err:%v", dropStmt, e)
		return e
	}
	stmt := fmt.Sprintf("CREATE TABLE %s LIFECYCLE %d AS SELECT * FROM %s WHERE %s", target, temporaryTableLifecycle, origin, cond)
	if _, e := db.Exec(stmt); e != nil {
		log.Errorf("create table failed, stmt:[%s], err:%v", stmt, e)
		return e
	}
	return nil
}

func namingTrainAndValDataset(origTable string) *trainAndValDataset {
	return &trainAndValDataset{
		supported:  true,
		table:      fmt.Sprintf("%s_%s_%s", tablePrefix, origTable, suffix),
		training:   fmt.Sprintf("%s_%s_%s", trainingPrefix, origTable, suffix),
		validation: fmt.Sprintf("%s_%s_%s", validationPrefix, origTable, suffix),
	}
}
