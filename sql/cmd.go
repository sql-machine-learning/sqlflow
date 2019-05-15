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
	}
	// TODO(weiguo): need an `else` to support maxCompute ?
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
		cmd = exec.Command("python")
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
