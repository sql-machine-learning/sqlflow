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

package main

import (
	"fmt"
	"os"

	"sqlflow.org/sqlflow/go/database"
	"sqlflow.org/sqlflow/go/model"
)

func downloadModelFromDB(opts *options) error {
	if opts.DataSource == "" {
		opts.DataSource = os.Getenv("SQLFLOW_DATASOURCE")
		if opts.DataSource == "" {
			return fmt.Errorf("You should specify a datasource with -d or set env SQLFLOW_DATASOURCE")
		}
	}
	db, err := database.OpenDB(opts.DataSource)
	if err != nil {
		return err
	}
	defer db.Close()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	filename, err := model.DumpDBModel(db, opts.ModelName, cwd)
	if err != nil {
		return err
	}
	fmt.Printf("model \"%s\" downloaded successfully at %s\n", opts.ModelName, filename)
	return nil
}
