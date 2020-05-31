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

package modelzooserver

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func imageExistsOnRegistry(imageName, tag string) bool {
	var imageNamePart string
	var registryPart string
	registryPart = os.Getenv("SQLFLOW_MODEL_ZOO_REGISTRY")
	imageNamePart = imageName
	// if the imageName contains domain name like "hub.docker.com/group/my_model",
	// get the registry and image name from input imageName.
	if strings.Contains(imageName, ".") {
		parts := strings.Split(imageName, "/")
		registryPart = parts[0]
		imageNamePart = strings.Join(parts[1:len(parts)-1], "/")
	}
	checkURL := fmt.Sprintf("https://%s/v1/repositories/%s/tags/%s", registryPart, imageNamePart, tag)
	resp, err := http.Get(checkURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	return true
}

func buildAndPushImage(dir, name, tag string, dryrun bool) error {
	// for public model zoo server, the Docker image name should not contain url prefix like
	// hub.docker.com/group/my_model_image
	if strings.Contains(name, ".") {
		return fmt.Errorf("release model definition to public model zoo server should not contain url prefix like hub.docker.com/group/my_model_image, the registry is configured at model zoo server")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	if err := os.Chdir(dir); err != nil {
		return err
	}

	cmd := exec.Command("docker", "build", ".", "-t", fmt.Sprintf("%s:%s", name, tag))
	var cmdStderr bytes.Buffer
	cmd.Stderr = &cmdStderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("run docker build err: %v, stderr: %s", err, cmdStderr.String())
	}

	if dryrun {
		// skip push to registry when dryrun enabled.
		return nil
	}

	dockerUsername := os.Getenv("SQLFLOW_MODEL_ZOO_REGISTRY_USER")
	dockerPasswd := os.Getenv("SQLFLOW_MODEL_ZOO_REGISTRY_PASS")
	if dockerUsername == "" || dockerPasswd == "" {
		return fmt.Errorf("need to set SQLFLOW_MODEL_ZOO_REGISTRY_USER and SQLFLOW_MODEL_ZOO_REGISTRY_PASS for model zoo server")
	}

	cmd = exec.Command("docker", "login", "-u", dockerUsername, "--password-stdin")
	cmd.Stderr = &cmdStderr
	cmd.Stdin = bytes.NewBufferString(dockerPasswd)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("run docker login err: %v, stderr: %s", err, cmdStderr.String())
	}

	cmd = exec.Command("docker", "push", fmt.Sprintf("%s:%s", name, tag))
	cmd.Stderr = &cmdStderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("run docker push err: %v, stderr: %s", err, cmdStderr.String())
	}
	return nil
}
