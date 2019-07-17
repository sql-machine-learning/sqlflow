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
	"os/exec"
)

func tryRun(cmd string, args ...string) bool {
	return exec.Command(cmd, args...).Run() == nil
}

func hasPython() bool {
	return tryRun("python", "-V")
}

func hasTensorFlow() bool {
	return tryRun("python", "-c", "import tensorflow")
}

func hasDatabaseConnector(driverName string) bool {
	if driverName == "hive" {
		return tryRun("python", "-c", "from impala.dbapi import connect")
	} else if driverName == "mysql" {
		return tryRun("python", "-c", "from mysql.connector import connect")
	} else if driverName == "sqlite3" {
		return tryRun("python", "-c", "from sqlite3 import connect")
	} else if driverName == "maxcompute" {
		return tryRun("python", "-c", "from odps import ODPS")
	}
	return false
}

func hasDocker() bool {
	return tryRun("docker", "version")
}

func hasDockerImage(image string) bool {
	b, e := exec.Command("docker", "images", "-q", image).Output()
	if e != nil || len(b) == 0 {
		return false
	}
	return true
}

func tensorflowCmd(cwd, driverName string) (cmd *exec.Cmd) {
	if hasPython() && hasTensorFlow() && hasDatabaseConnector(driverName) {
		log.Printf("tensorflowCmd: run locally")
		cmd = exec.Command("python", "-u")
		cmd.Dir = cwd
	} else if hasDocker() {
		log.Printf("tensorflowCmd: run in Docker container")
		const tfImg = "sqlflow/sqlflow"
		if !hasDockerImage(tfImg) {
			log.Printf("No local Docker image %s.  It will take a long time to pull.", tfImg)
		}
		cmd = exec.Command("docker", "run", "--rm",
			fmt.Sprintf("-v%s:/work", cwd),
			"-w/work", "--network=host", "-i", tfImg, "python")
	} else {
		log.Fatalf("No local TensorFlow or Docker.  No way to run TensorFlow programs")
	}
	return cmd
}
