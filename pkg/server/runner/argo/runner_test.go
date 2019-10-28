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

package argo

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	wfclientset "github.com/argoproj/argo/pkg/client/clientset/versioned"
	"github.com/argoproj/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	helloWorldWorkflow = wfv1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "hello-world-",
		},
		Spec: wfv1.WorkflowSpec{
			Entrypoint: "whalesay",
			Templates: []wfv1.Template{
				{
					Name: "whalesay",
					Container: &corev1.Container{
						Image:   "docker/whalesay:latest",
						Command: []string{"cowsay", "hello world"},
					},
				},
			},
		},
	}
)

func TestArgoClient(t *testing.T) {
	jobRunner := os.Getenv("SQLFLOW_job_runner")
	if jobRunner != "argo" {
		t.Skip("Skip, this test is for Argo")
	}

	// get current user to determine home directory
	usr, err := user.Current()
	checkErr(err)

	// get kubeconfig file location
	kubeconfig := flag.String("kubeconfig", filepath.Join(usr.HomeDir, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	checkErr(err)
	namespace := "default"

	// create the workflow client
	wfClient := wfclientset.NewForConfigOrDie(config).ArgoprojV1alpha1().Workflows(namespace)

	// submit the hello world workflow
	createdWf, err := wfClient.Create(&helloWorldWorkflow)
	checkErr(err)
	fmt.Printf("Workflow %s submitted\n", createdWf.Name)

	// wait for the workflow to complete
	fieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", createdWf.Name))
	watchIf, err := wfClient.Watch(metav1.ListOptions{FieldSelector: fieldSelector.String()})
	errors.CheckError(err)
	defer watchIf.Stop()
	for next := range watchIf.ResultChan() {
		wf, ok := next.Object.(*wfv1.Workflow)
		if !ok {
			continue
		}
		if !wf.Status.FinishedAt.IsZero() {
			fmt.Printf("Workflow %s %s at %v\n", wf.Name, wf.Status.Phase, wf.Status.FinishedAt)
			break
		}
	}
}

func checkErr(err error) {
	if err != nil {
		panic(err.Error())
	}
}
