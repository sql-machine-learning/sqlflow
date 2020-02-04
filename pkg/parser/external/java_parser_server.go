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
	"fmt"
	"google.golang.org/grpc"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

var (
	mu         sync.Mutex
	clientConn *grpc.ClientConn
)

func getServerPort() string {
	port := os.Getenv("SQLFLOW_PARSER_SERVER_PORT")
	if port == "" {
		log.Fatal("undefined environment variable SQLFLOW_PARSER_SERVER_PORT")
	}
	return port
}

func isServerUp() bool {
	cmd := exec.Command("curl", "-v", fmt.Sprintf("localhost:%s", getServerPort()))
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func startServerIfNotUp() error {
	if isServerUp() {
		return nil
	}

	cmd := exec.Command("java",
		"-cp", "/opt/sqlflow/parser/parser-1.0-SNAPSHOT-jar-with-dependencies.jar",
		"org.sqlflow.parser.ParserGrpcServer",
		"-p", getServerPort())
	if err := cmd.Start(); err != nil {
		return err
	}

	for i := 0; i < 3; i++ {
		time.Sleep(time.Second)
		if isServerUp() {
			return nil
		}
	}

	return fmt.Errorf("unable to start external parser service")
}

func connectToServer() (*grpc.ClientConn, error) {
	mu.Lock()
	defer mu.Unlock()

	if clientConn != nil {
		return clientConn, nil
	}

	if err := startServerIfNotUp(); err != nil {
		return nil, err
	}

	dialAddress := fmt.Sprintf(":%s", getServerPort())
	clientConn, err := grpc.Dial(dialAddress, grpc.WithInsecure())
	return clientConn, err
}
