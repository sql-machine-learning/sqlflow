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
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"sqlflow.org/sqlflow/go/randstring"
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

func buildAndPushImageKaniko(dir, name, tag string, dryrun bool) error {
	reg := os.Getenv("SQLFLOW_MODEL_ZOO_REGISTRY")
	// default push to dockerhub
	if reg == "" {
		reg = "https://index.docker.io/v1/"
	}
	regUsername := os.Getenv("SQLFLOW_MODEL_ZOO_REGISTRY_USER")
	regPasswd := os.Getenv("SQLFLOW_MODEL_ZOO_REGISTRY_PASS")
	regEmail := os.Getenv("SQLFLOW_MODEL_ZOO_REGISTRY_EMAIL")
	if regUsername == "" || regPasswd == "" || regEmail == "" {
		return fmt.Errorf("need to set SQLFLOW_MODEL_ZOO_REGISTRY_USER and SQLFLOW_MODEL_ZOO_REGISTRY_PASS for model zoo server")
	}
	regSecret := fmt.Sprintf("kaniko-regcred-%s", strings.ToLower(randstring.Generate(8)))

	// TODO(typhoonzero): support any registry servers
	// TODO(typhoonzero): use a random
	// create docker registry credentials
	cmd := fmt.Sprintf(`kubectl create secret docker-registry %s --docker-server=%s --docker-username=%s --docker-password=%s --docker-email=%s`, regSecret, reg, regUsername, regPasswd, regEmail)
	cmdList := strings.Split(cmd, " ")
	c := exec.Command(cmdList[0], cmdList[1:]...)
	if err := c.Run(); err != nil {
		return err
	}
	defer func() error {
		cmdDeleteSecret := exec.Command("kubectl", "delete", "secret", regSecret)
		if err := cmdDeleteSecret.Run(); err != nil {
			return err
		}
		return nil
	}()

	// cd into dir to tar the context
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)
	if err := os.Chdir(dir); err != nil {
		return err
	}

	kanikoImage := os.Getenv("SQLFLOW_MODEL_ZOO_KANIKO_IMAGE")
	if kanikoImage == "" {
		kanikoImage = "registry.cn-hangzhou.aliyuncs.com/sql-machine-learning/kaniko-executor"
	}
	kanikoPodName := fmt.Sprintf("kaniko-%s", strings.ToLower(randstring.Generate(8)))
	destination := fmt.Sprintf("%s:%s", name, tag)
	// pod tolerations/nodeSelector JSON like: '"tolerations": [...],', if empty string, tolerations will not be set.
	tolerations := os.Getenv("SQLFLOW_MODEL_ZOO_TOLERATIONS")
	nodeSelector := os.Getenv("SQLFLOW_MODEL_ZOO_NODE_SELECTOR")
	podTemplate := fmt.Sprintf(`'{
  "apiVersion": "v1",
  "spec": {
    %s
    %s
    "containers": [
    {
      "name": "%s",
      "image": "%s",
      "stdin": true,
      "stdinOnce": true,
      "args": [
        "--dockerfile=Dockerfile",
        "--context=tar://stdin",
        "--force",
        "--destination=%s"],
      "volumeMounts": [
        {
          "name": "kaniko-secret",
          "mountPath": "/kaniko/.docker/"
      }]
    }],
    "volumes": [
    {
      "name": "kaniko-secret",
      "secret": {
        "secretName": "%s",
        "items": [{"key": ".dockerconfigjson", "path": "config.json"}]
      }}
    ]
  }
}'`, tolerations, nodeSelector, kanikoPodName, kanikoImage, destination, regSecret)

	tarContextCmd := exec.Command("tar", "czf", "-", ".")
	// exec.Command can not handle quotes correctly, use bach -c here.
	kanikoBuildCmdStr := fmt.Sprintf(`kubectl run %s --rm --stdin=true --restart=Never --image=%s --overrides=%s`, kanikoPodName, kanikoImage, podTemplate)
	kanikoBuildCmdStdin := exec.Command("bash", "-c", kanikoBuildCmdStr)

	r, w := io.Pipe()
	tarContextCmd.Dir = dir
	tarContextCmd.Stdout = w
	kanikoBuildCmdStdin.Stdin = r
	var outputErr bytes.Buffer
	kanikoBuildCmdStdin.Stderr = &outputErr
	if err := tarContextCmd.Start(); err != nil {
		return err
	}
	if err := kanikoBuildCmdStdin.Start(); err != nil {
		return err
	}
	if err := tarContextCmd.Wait(); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	if err := kanikoBuildCmdStdin.Wait(); err != nil {
		// TODO(typhoonzero): use log to output error messages.
		io.Copy(os.Stdout, &outputErr)
		return err
	}

	return nil
}
