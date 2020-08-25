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

package sql

import (
	"net/url"
	"os"
	"strings"

	db "sqlflow.org/sqlflow/go/database"
	pb "sqlflow.org/sqlflow/go/proto"
)

// MakeSessionFromEnv returns proto.Session which comes from the environment variables
func MakeSessionFromEnv() *pb.Session {
	session := &pb.Session{
		Token:        os.Getenv("SQLFLOW_USER_TOKEN"),
		DbConnStr:    os.Getenv("SQLFLOW_DATASOURCE"),
		ExitOnSubmit: strings.ToLower(os.Getenv("SQLFLOW_EXIT_ON_SUBMIT")) == "true",
		UserId:       os.Getenv("SQLFLOW_USER_ID"),
		Submitter:    os.Getenv("SQLFLOW_submitter")}

	// User should specify hive params in uri,
	// to stay compatible with historical logic,
	// we assemble hive envrionment params into uri
	hiveParams := map[string]string{
		"hive_location":      os.Getenv("SQLFLOW_HIVE_LOCATION"),
		"hdfs_namenode_addr": os.Getenv("SQLFLOW_HDFS_NAMENODE_ADDR"),
	}
	hdfsUser := os.Getenv("SQLFLOW_HADOOP_USER")
	hdfsPass := os.Getenv("SQLFLOW_HADOOP_PASS")

	driver, _, e := db.ParseURL(session.DbConnStr)
	if e != nil || driver != "hive" {
		return session
	}
	uri, e := url.Parse(session.DbConnStr)
	if e != nil {
		return session
	}
	if uri.User == nil && hdfsUser != "" {
		uri.User = url.UserPassword(hdfsUser, hdfsPass)
	}
	query := uri.Query()
	for k, v := range hiveParams {
		if query.Get(k) == "" && v != "" {
			query.Set(k, v)
		}
	}
	uri.RawQuery = query.Encode()
	session.DbConnStr = uri.String()

	return session
}
