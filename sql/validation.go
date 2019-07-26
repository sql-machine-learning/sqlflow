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
	"time"

	"github.com/pkg/errors"
)

type trainingDataset struct {
	// `supported` here identifies if SQLFlow is able to split dataset into
	// training dataset and validation dataset.
	// So, TODO(weiguo): Let's remove `supproted` if SQLFlow supports other
	// drivers, like: MySQL, hive(specified in database.go:open()).
	supported      bool
	table          string
	trainingView   string // view of table (<k)
	validationView string // view of table (>=k)
}

const (
	temporaryTableLifecycle = 14 // day(s)
	randomColumn            = "sqlflow_rdm"
	tablePrefix             = "sqlflow_tv_" // 'tv' = training & validation
	trainingViewPrefix      = "sqlflow_view_training_"
	validationViewPrefix    = "sqlflow_view_validation_"
)

var (
	errBadBoundary = errors.New("boundary should between (0.0, 1.0) exclude")
)

// SQLFlow generates a temporary table, + sqlflow_randowm column
func createTrainingDataset(db *DB, slct string, trainingUpperbound float32) (trainingDataset, error) {
	if trainingUpperbound <= 0 || trainingUpperbound >= 1 {
		return trainingDataset{}, errBadBoundary
	}

	switch db.driverName {
	case "maxcompute":
		return createMaxcomputeDataset(db, slct, trainingUpperbound)
		// TODO(weiguo): support other databases, like: "hive", "mysql"...
	default:
		return trainingDataset{}, nil
	}
}

func releaseTrainingDataset(ds trainingDataset) {
	// TODO(weiguo): release resources for databases, like: "hive", "mysql"...
}

func createMaxcomputeDataset(db *DB, slct string, trainingUpperbound float32) (trainingDataset, error) {
	ds := namingTrainingDataset()
	// create a table, then split it into 2 views
	stmt := fmt.Sprintf("CREATE TABLE %s LIFECYCLE %d AS SELECT *, RAND() AS %s FROM (%s) AS %s_ori", ds.table, temporaryTableLifecycle, randomColumn, slct, ds.table)
	if _, e := db.Exec(stmt); e != nil {
		log.Errorf("create temporary table failed, stmt:[%s], err:%v", stmt, e)
		return trainingDataset{}, e
	}
	trainingCond := fmt.Sprintf("%s < %f", randomColumn, trainingUpperbound)
	if e := createView(db, ds.table, ds.trainingView, trainingCond); e != nil {
		return trainingDataset{}, e
	}
	validationCond := fmt.Sprintf("%s >= %f", randomColumn, trainingUpperbound)
	if e := createView(db, ds.table, ds.validationView, validationCond); e != nil {
		return trainingDataset{}, e
	}
	return ds, nil
}

func createView(db *DB, table, view, where string) error {
	stmt := fmt.Sprintf("CREATE VIEW %s AS SELECT * FROM %s WHERE %s", view, table, where)
	if _, e := db.Exec(stmt); e != nil {
		log.Errorf("create view failed, stmt:[%s], err:%v", stmt, e)
		return e
	}
	return nil
}

func namingTrainingDataset() trainingDataset {
	uniq := time.Now().UnixNano() / 1e3
	return trainingDataset{
		supported:      true,
		table:          fmt.Sprintf("%s%d", tablePrefix, uniq),
		trainingView:   fmt.Sprintf("%s%d", trainingViewPrefix, uniq),
		validationView: fmt.Sprintf("%s%d", validationViewPrefix, uniq),
	}
}
