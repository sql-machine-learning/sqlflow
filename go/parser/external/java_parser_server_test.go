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
	"github.com/stretchr/testify/assert"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func getFreePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func TestIsServerUp(t *testing.T) {
	a := assert.New(t)

	port, err := getFreePort()
	a.NoError(err)

	address := fmt.Sprintf(":%d", port)
	a.False(isServerUp(address))

	// start Java parser server
	cmd := exec.Command("java",
		"-cp", filepath.Join(getServerLoadingPath(), "parser-1.0-SNAPSHOT-jar-with-dependencies.jar"),
		"org.sqlflow.parser.ParserGrpcServer",
		"-p", strconv.Itoa(port),
		"-l", getServerLoadingPath())
	err = cmd.Start()
	a.NoError(err)

	// server should be up within 5 seconds
	isUp := false
	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		if isUp = isServerUp(address); isUp {
			break
		}
	}
	a.True(isUp)

	err = cmd.Process.Kill()
	a.NoError(err)
}
