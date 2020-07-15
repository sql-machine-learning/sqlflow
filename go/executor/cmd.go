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

package executor

import (
	"fmt"
	"log"
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
		return tryRun("python", "-c", "from MySQLdb import connect")
	} else if driverName == "maxcompute" {
		return tryRun("python", "-c", "from odps import ODPS")
	}
	return false
}

func hasDocker() bool {
	return tryRun("docker", "version")
}

func hasElasticDLCmd() bool {
	return tryRun("elasticdl", "-h")
}

func hasDockerImage(image string) bool {
	b, e := exec.Command("docker", "images", "-q", image).Output()
	if e != nil || len(b) == 0 {
		return false
	}
	return true
}

func sqlflowCmd(cwd, driverName string) (cmd *exec.Cmd) {
	if hasPython() && hasTensorFlow() && hasDatabaseConnector(driverName) {
		cmd = exec.Command("python", "-u")
		cmd.Dir = cwd
	} else if hasDocker() {
		const tfImg = "sqlflow/sqlflow"
		if !hasDockerImage(tfImg) {
			// TODO(yancey1989): write log into pipe to avoid the wrong row/
			//log.Printf("sqlflowCmd: No local Docker image %s.  It will take a long time to pull.", tfImg)
		}
		cmd = exec.Command("docker", "run", "--rm",
			fmt.Sprintf("-v%s:/work", cwd),
			"-w/work", "--network=host", "-i", tfImg, "python")
	} else if hasPython() {
		// NOTE: some docker images (for example server images on Dataworks) do not
		// install TensorFlow and Docker. Just run the Python code directly.
		cmd = exec.Command("python", "-u")
		cmd.Dir = cwd
	} else {
		log.Fatalf("No local TensorFlow, Docker and Python.  No way to run the program")
	}
	return cmd
}
