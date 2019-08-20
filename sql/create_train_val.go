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
	"strings"

	"github.com/pkg/errors"
)

type trainAndValDataset struct {
	// `supported` here identifies if SQLFlow is able to split dataset into
	// training dataset and validation dataset.
	// So, TODO(weiguo): Let's remove `supproted` if SQLFlow supports other
	// drivers, like: MySQL, hive(specified in database.go:open()).
	supported  bool
	database   string
	table      string
	training   string // table for training: < k
	validation string // table for validation: >= k
}

const (
	temporaryTableLifecycle = 14 // day(s), for maxcompuate
	randomColumn            = "sqlflow_rdm"
	tablePrefix             = "sqlflow_tv" // 'tv' = training & validation
	trainingPrefix          = "sqlflow_training"
	validationPrefix        = "sqlflow_validation"
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
	case "hive", "mysql":
		return createDataset(db, slct, origTable, trainingUpperbound)
	// TODO(weiguo) case "sqlite":
	default:
		return nil, nil
	}
}

func createMaxcomputeDataset(db *DB, slct string, origTable string, trainingUpperbound float32) (*trainAndValDataset, error) {
	ds := namingTrainAndValDataset(origTable)
	if e := createMaxcomputeRandomTable(ds.table, slct, db); e != nil {
		log.Errorf("create table with a randowm column failed, err: %v", e)
		return nil, e
	}
	trnCond := fmt.Sprintf("%s < %f", randomColumn, trainingUpperbound)
	if e := createMaxcomputeTable(ds.training, ds.table, db, trnCond); e != nil {
		log.Errorf("create training table failed, err: %v", e)
		return nil, e
	}
	valCond := fmt.Sprintf("%s >= %f", randomColumn, trainingUpperbound)
	if e := createMaxcomputeTable(ds.validation, ds.table, db, valCond); e != nil {
		log.Errorf("create validation table failed, err: %v", e)
		return nil, e
	}
	// TODO(weiguo): release the random table
	return ds, nil
}

func createMaxcomputeRandomTable(target, slct string, db *DB) error {
	// drop the table if already exist
	dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", target)
	if _, e := db.Exec(dropStmt); e != nil {
		return e
	}
	// create a table, then split it into train and val tables
	stmt := fmt.Sprintf("CREATE TABLE %s LIFECYCLE %d AS SELECT *, RAND() AS %s FROM (%s) AS %s_ori", target, temporaryTableLifecycle, randomColumn, slct, target)
	_, e := db.Exec(stmt)
	return e
}

func createMaxcomputeTable(target, origin string, db *DB, cond string) error {
	dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", target)
	if _, e := db.Exec(dropStmt); e != nil {
		return e
	}
	stmt := fmt.Sprintf("CREATE TABLE %s LIFECYCLE %d AS SELECT * FROM %s WHERE %s", target, temporaryTableLifecycle, origin, cond)
	_, e := db.Exec(stmt)
	return e
}

// create dataset on Hive, MySQL
func createDataset(db *DB, slct string, origTable string, trainingUpperbound float32) (*trainAndValDataset, error) {
	ds := namingTrainAndValDataset(origTable)
	stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", ds.database)
	if _, e := db.Exec(stmt); e != nil {
		log.Errorf("create temporary database failed, stmt:[%s], err:%v", stmt, e)
		return nil, e
	}
	rdmTbl, e := createRandomTable(ds.database, ds.table, slct, db)
	if e != nil {
		log.Errorf("create table with a random column failed, err: %v", e)
		return nil, e
	}
	trnCond := fmt.Sprintf("%s < %f", randomColumn, trainingUpperbound)
	trnTbl, e := createTable(ds.database, ds.training, rdmTbl, db, trnCond)
	if e != nil {
		log.Errorf("create training table failed, err: %v", e)
		return nil, e
	}
	ds.training = trnTbl

	valCond := fmt.Sprintf("%s >= %f", randomColumn, trainingUpperbound)
	valTbl, e := createTable(ds.database, ds.validation, rdmTbl, db, valCond)
	if e != nil {
		log.Errorf("create validation table failed, err: %v", e)
		return nil, e
	}
	ds.validation = valTbl
	if _, e := db.Exec("DROP TABLE IF EXISTS " + rdmTbl); e != nil {
		log.Errorf("drop temporary table failed, err:%v", e)
		return nil, e
	}
	return ds, nil
}

func createRandomTable(database, table, slct string, db *DB) (string, error) {
	fullTbl := fmt.Sprintf("%s.%s", database, table)
	dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", fullTbl)
	if _, e := db.Exec(dropStmt); e != nil {
		return "", e
	}
	stmt := fmt.Sprintf("CREATE TABLE %s AS SELECT *, RAND() AS %s FROM (%s) AS %s_ori", fullTbl, randomColumn, slct, table)
	_, e := db.Exec(stmt)
	return fullTbl, e
}

func createTable(database, table, origin string, db *DB, cond string) (string, error) {
	fullTbl := fmt.Sprintf("%s.%s", database, table)
	dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", fullTbl)
	if _, e := db.Exec(dropStmt); e != nil {
		return "", e
	}
	stmt := fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM %s WHERE %s", fullTbl, origin, cond)
	_, e := db.Exec(stmt)
	return fullTbl, e
}

func namingTrainAndValDataset(origTable string) *trainAndValDataset {
	// hive returns a table with a database name
	flattenTbl := strings.Replace(origTable, ".", "__", -1)
	return &trainAndValDataset{
		supported:  true,
		database:   "sf_home",
		table:      fmt.Sprintf("%s_%s", tablePrefix, flattenTbl),
		training:   fmt.Sprintf("%s_%s", trainingPrefix, flattenTbl),
		validation: fmt.Sprintf("%s_%s", validationPrefix, flattenTbl),
	}
}

func releaseTrainAndValDataset(db *DB, ds *trainAndValDataset) error {
	switch db.driverName {
	case "hive", "mysql":
		if _, e := db.Exec("DROP TABLE IF EXISTS " + ds.training); e != nil {
			return e
		}
		if _, e := db.Exec("DROP TABLE IF EXISTS " + ds.validation); e != nil {
			return e
		}
	default:
	}
	return nil
}
