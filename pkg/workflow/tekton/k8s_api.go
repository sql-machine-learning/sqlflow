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

package tekton

import (
	"fmt"

	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tektoncli "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client struct packages Kubernetes and Tekton Client
type Client struct {
	namespace    string
	k8sclient    *kubernetes.Clientset
	tektonclient *tektoncli.TektonV1alpha1Client
}

func (c *Client) getTaskRun(name string) (*tektonapi.TaskRun, error) {
	fmt.Println(c.namespace, name)
	tr := c.tektonclient.TaskRuns(c.namespace)
	return tr.Get(name, metav1.GetOptions{})
}

func (c *Client) getPod(name string) (*corev1.Pod, error) {
	pod := c.k8sclient.CoreV1().Pods(c.namespace)
	return pod.Get(name, metav1.GetOptions{})
}

func newClient() (*Client, error) {
	c := &Client{}
	var e error

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	c.namespace, _, e = kubeConfig.Namespace()
	if e != nil {
		return nil, e
	}

	config, e := kubeConfig.ClientConfig()
	if e != nil {
		return nil, e
	}

	c.k8sclient, e = kubernetes.NewForConfig(config)
	if e != nil {
		return nil, e
	}

	c.tektonclient, e = tektoncli.NewForConfig(config)
	if e != nil {
		return nil, e
	}

	return c, nil
}

func isPodPending(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodPending
}
