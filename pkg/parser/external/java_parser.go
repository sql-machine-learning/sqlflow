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

package external

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"os"
	"os/exec"
	"sqlflow.org/sqlflow/pkg/proto"
	"time"
)

type javaParser struct {
	typ string
}

// typ should be either "hiveql" or "calcite".
func newJavaParser(typ string) *javaParser {
	return &javaParser{typ: typ}
}

func getServerAddress() string {
	ip := getEnv("SQLFLOW_EXTERNAL_PARSER_IP", "localhost")
	port := getEnv("SQLFLOW_EXTERNAL_PARSER_PORT", "12345")
	return fmt.Sprintf("%s:%s", ip, port)
}

func isServerUp(address string) bool {
	cmd := exec.Command("curl", "-v", address)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func startServerIfNotUp() error {
	address := getServerAddress()
	if isServerUp(address) {
		return nil
	}

	port := getEnv("SQLFLOW_EXTERNAL_PARSER_PORT", "12345")
	cmd := exec.Command("java",
		"-cp", "/opt/sqlflow/parser/parser-1.0-SNAPSHOT-jar-with-dependencies.jar",
		"org.sqlflow.parser.ParserGrpcServer",
		"-p", port)
	if err := cmd.Start(); err != nil {
		return err
	}

	for i := 0; i < 3; i++ {
		time.Sleep(time.Second)
		if isServerUp(address) {
			return nil
		}
	}

	return fmt.Errorf("unable to start external parser service")
}

func (p *javaParser) Parse(program string) ([]string, int, error) {
	if err := startServerIfNotUp(); err != nil {
		return nil, -1, err
	}

	c, err := grpc.Dial(getServerAddress(), grpc.WithInsecure())
	if err != nil {
		return nil, -1, err
	}
	defer c.Close()

	r, err := proto.NewParserClient(c).Parse(context.Background(), &proto.ParserRequest{Dialect: p.typ, SqlProgram: program})
	if err != nil {
		return nil, -1, err
	}
	if r.Error != "" {
		return nil, -1, fmt.Errorf(r.Error)
	}
	return r.SqlStatements, int(r.Index), nil
}

func getEnv(name, fallback string) string {
	if os.Getenv(name) != "" {
		return os.Getenv(name)
	}
	return fallback
}
