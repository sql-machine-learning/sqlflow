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
	k8sHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	k8sPort := os.Getenv("KUBERNETES_SERVICE_PORT")
	if k8sHost == "" || k8sPort == "" {
		return fmt.Errorf("buildAndPushImageKaniko must be called when model zoo server is deployed in Kubernetes cluster")
	}
	dockerUsername := os.Getenv("SQLFLOW_MODEL_ZOO_REGISTRY_USER")
	dockerPasswd := os.Getenv("SQLFLOW_MODEL_ZOO_REGISTRY_PASS")
	if dockerUsername == "" || dockerPasswd == "" {
		return fmt.Errorf("need to set SQLFLOW_MODEL_ZOO_REGISTRY_USER and SQLFLOW_MODEL_ZOO_REGISTRY_PASS for model zoo server")
	}

	// TODO(typhoonzero): support any registry servers
	// TODO(typhoonzero): use a random
	// create docker registry credentials
	cmd := fmt.Sprintf(`kubectl create secret docker-registry kaniko-regcred --docker-server=https://index.docker.io/v1/ --docker-username=%s --docker-password=%s`, dockerUsername, dockerPasswd)
	cmdList := strings.Split(cmd, " ")
	c := exec.Command(cmdList[0], cmdList[1:]...)
	fmt.Println(cmd)
	if err := c.Run(); err != nil {
		return err
	}

	destination := fmt.Sprintf("%s:%s", name, tag)
	podTemplate := fmt.Sprintf(`'{
  "apiVersion": "v1",
  "spec": {
    "containers": [
    {
      "name": "kaniko",
      "image": "daocloud.io/gcr-mirror/kaniko-project-executor:latest",
      "stdin": true,
      "stdinOnce": true,
      "args": [
        "--dockerfile=Dockerfile",
        "--context=tar://stdin",
        "--destination=%s" ],
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
		  "secretName": "kaniko-regcred",
		  "items": [{"key": ".dockerconfigjson", "path": "config.json"}]
	  }}
    ]
  }
}'`, destination)

	tarContextCmd := exec.Command("tar", "-czf", "-", dir)
	kanikoBuildCmdStdin := exec.Command("kubectl", "run", "kaniko", "--rm",
		"--stdin=true",
		"--image=daocloud.io/gcr-mirror/kaniko-project-executor:latest",
		fmt.Sprintf("--overrides=%s", podTemplate))
	r, w := io.Pipe()
	tarContextCmd.Stdout = w
	kanikoBuildCmdStdin.Stdin = r
	var output bytes.Buffer
	var outputErr bytes.Buffer
	kanikoBuildCmdStdin.Stdout = &output
	kanikoBuildCmdStdin.Stderr = &outputErr
	if err := tarContextCmd.Start(); err != nil {
		fmt.Println("tarContextCmd.Start")
		return err
	}
	if err := kanikoBuildCmdStdin.Start(); err != nil {
		fmt.Println("kanikoBuildCmdStdin.Start")
		return err
	}
	if err := tarContextCmd.Wait(); err != nil {
		fmt.Println("tarContextCmd.Wait")
		return err
	}
	if err := w.Close(); err != nil {
		fmt.Println("w.Close")
		return err
	}

	if err := kanikoBuildCmdStdin.Wait(); err != nil {
		io.Copy(os.Stdout, &outputErr)
		fmt.Println("kanikoBuildCmdStdin.Wait")
		return err
	}

	cmdDeleteSecret := exec.Command("kubectl", "delete", "secret", "kaniko-regcred")
	if err := cmdDeleteSecret.Run(); err != nil {
		return err
	}
	return nil
}
