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
	"os"
	pb "sqlflow.org/sqlflow/pkg/proto"
	"strings"
)

// MakeSessionFromEnv returns proto.Session which comes from the environment variables
func MakeSessionFromEnv() *pb.Session {
	return &pb.Session{
		Token:            os.Getenv("SQLFLOW_USER_TOKEN"),
		DbConnStr:        os.Getenv("SQLFLOW_DATASOURCE"),
		ExitOnSubmit:     strings.ToLower(os.Getenv("SQLFLOW_EXIT_ON_SUBMIT")) == "true",
		UserId:           os.Getenv("SQLFLOW_USER_ID"),
		HiveLocation:     os.Getenv("SQLFLOW_HIVE_LOCATION"),
		HdfsNamenodeAddr: os.Getenv("SQLFLOW_HDFS_NAMENODE_ADDR"),
		HdfsUser:         os.Getenv("SQLFLOW_HADOOP_USER"),
		HdfsPass:         os.Getenv("SQLFLOW_HADOOP_PASS"),
		Submitter:        os.Getenv("SQLFLOW_submitter")}
}
