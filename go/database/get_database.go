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

package database

import (
	"log"
	"regexp"

	"sqlflow.org/goalisa"
	"sqlflow.org/gohive"
	"sqlflow.org/gomaxcompute"
)

// GetDatabaseName parse the database name (or MaxCompute project) from the database connection string
func GetDatabaseName(datasource string) (string, error) {
	driver, dsName, e := ParseURL(datasource)
	if e != nil {
		return "", e
	}
	switch driver {
	case "maxcompute":
		// maxcompute://root:root@odps.com?curr_project=my_project
		cfg, e := gomaxcompute.ParseDSN(dsName)
		if e != nil {
			return "", e
		}
		return cfg.Project, nil
	case "alisa":
		// alisa://root:root@dataworks.com?curr_project=my_project
		cfg, e := goalisa.ParseDSN(dsName)
		if e != nil {
			return "", e
		}
		return cfg.Project, nil
	case "mysql":
		// mysql://root:root@tcp(127.0.0.1:3306)/iris?maxAllowedPacket=0
		re := regexp.MustCompile(`[^/]*/(\w*).*`) // Extract the database name of MySQL and Hive
		if group := re.FindStringSubmatch(dsName); group != nil {
			return group[1], nil
		}
	case "hive":
		// hive://root:root@127.0.0.1:10000/iris?auth=NOSASL
		cfg, e := gohive.ParseDSN(dsName)
		if e != nil {
			return "", e
		}
		return cfg.DBName, nil
	default:
		log.Fatalf("unknown database '%s' in data source'%s'", driver, datasource)
	}
	return "", nil
}
