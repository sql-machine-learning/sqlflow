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

package argo

import (
	"os"
	"testing"

	wfv1 "github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	"github.com/stretchr/testify/assert"
)

const (
	testWorkflowDescription = `{
    "apiVersion": "argoproj.io/v1alpha1",
    "kind": "Workflow",
    "metadata": {
        "creationTimestamp": "2019-12-09T09:12:37Z",
        "generateName": "steps-",
        "generation": 11,
        "labels": {
            "workflows.argoproj.io/completed": "true",
            "workflows.argoproj.io/phase": "Succeeded"
        },
        "name": "steps-7lxxs",
        "namespace": "default",
        "resourceVersion": "1756798",
        "selfLink": "/apis/argoproj.io/v1alpha1/namespaces/default/workflows/steps-7lxxs",
        "uid": "708558fd-2637-4fb9-8cce-25f0a935639e"
    },
    "spec": {
        "arguments": {},
        "entrypoint": "hello-hello-hello",
        "templates": [
            {
                "arguments": {},
                "inputs": {},
                "metadata": {},
                "name": "hello-hello-hello",
                "outputs": {},
                "steps": [
                    [
                        {
                            "arguments": {
                                "parameters": [
                                    {
                                        "name": "message",
                                        "value": "hello1"
                                    }
                                ]
                            },
                            "name": "hello1",
                            "template": "whalesay"
                        }
                    ],
                    [
                        {
                            "arguments": {
                                "parameters": [
                                    {
                                        "name": "message",
                                        "value": "hello2"
                                    }
                                ]
                            },
                            "name": "hello2",
                            "template": "whalesay"
                        }
                    ],
                    [
                        {
                            "arguments": {
                                "parameters": [
                                    {
                                        "name": "message",
                                        "value": "hello1"
                                    }
                                ]
                            },
                            "name": "hello3",
                            "template": "whalesay"
                        }
                    ]
                ]
            },
            {
                "arguments": {},
                "container": {
                    "args": [
                        "{{inputs.parameters.message}}"
                    ],
                    "command": [
                        "cowsay"
                    ],
                    "image": "docker/whalesay",
                    "name": "",
                    "resources": {}
                },
                "inputs": {
                    "parameters": [
                        {
                            "name": "message"
                        }
                    ]
                },
                "metadata": {},
                "name": "whalesay",
                "outputs": {}
            }
        ]
    },
    "status": {
        "finishedAt": "2019-12-09T09:13:03Z",
        "nodes": {
            "steps-7lxxs": {
                "children": [
                    "steps-7lxxs-1184503397"
                ],
                "displayName": "steps-7lxxs",
                "finishedAt": "2019-12-09T09:13:03Z",
                "id": "steps-7lxxs",
                "name": "steps-7lxxs",
                "outboundNodes": [
                    "steps-7lxxs-1288663778"
                ],
                "phase": "Succeeded",
                "startedAt": "2019-12-09T09:12:37Z",
                "storedTemplateID": "/hello-hello-hello",
                "templateName": "hello-hello-hello",
                "type": "Steps"
            },
            "steps-7lxxs-1184503397": {
                "boundaryID": "steps-7lxxs",
                "children": [
                    "steps-7lxxs-2267726410"
                ],
                "displayName": "[0]",
                "finishedAt": "2019-12-09T09:12:44Z",
                "id": "steps-7lxxs-1184503397",
                "name": "steps-7lxxs[0]",
                "phase": "Succeeded",
                "startedAt": "2019-12-09T09:12:37Z",
                "templateName": "hello-hello-hello",
                "type": "StepGroup"
            },
            "steps-7lxxs-1263033216": {
                "boundaryID": "steps-7lxxs",
                "children": [
                    "steps-7lxxs-43331115"
                ],
                "displayName": "hello2",
                "finishedAt": "2019-12-09T09:12:50Z",
                "id": "steps-7lxxs-1263033216",
                "inputs": {
                    "parameters": [
                        {
                            "name": "message",
                            "value": "hello2"
                        }
                    ]
                },
                "name": "steps-7lxxs[1].hello2",
                "phase": "Succeeded",
                "startedAt": "2019-12-09T09:12:44Z",
                "storedTemplateID": "/whalesay",
                "templateName": "whalesay",
                "type": "Pod"
            },
            "steps-7lxxs-1288663778": {
                "boundaryID": "steps-7lxxs",
                "displayName": "hello3",
                "finishedAt": "2019-12-09T09:13:01Z",
                "id": "steps-7lxxs-1288663778",
                "inputs": {
                    "parameters": [
                        {
                            "name": "message",
                            "value": "hello1"
                        }
                    ]
                },
                "name": "steps-7lxxs[2].hello3",
                "phase": "Succeeded",
                "startedAt": "2019-12-09T09:12:52Z",
                "storedTemplateID": "/whalesay",
                "templateName": "whalesay",
                "type": "Pod"
            },
            "steps-7lxxs-2267726410": {
                "boundaryID": "steps-7lxxs",
                "children": [
                    "steps-7lxxs-43875568"
                ],
                "displayName": "hello1",
                "finishedAt": "2019-12-09T09:12:43Z",
                "id": "steps-7lxxs-2267726410",
                "inputs": {
                    "parameters": [
                        {
                            "name": "message",
                            "value": "hello1"
                        }
                    ]
                },
                "name": "steps-7lxxs[0].hello1",
                "phase": "Succeeded",
                "startedAt": "2019-12-09T09:12:37Z",
                "storedTemplateID": "/whalesay",
                "templateName": "whalesay",
                "type": "Pod"
            },
            "steps-7lxxs-43331115": {
                "boundaryID": "steps-7lxxs",
                "children": [
                    "steps-7lxxs-1288663778"
                ],
                "displayName": "[2]",
                "finishedAt": "2019-12-09T09:13:03Z",
                "id": "steps-7lxxs-43331115",
                "name": "steps-7lxxs[2]",
                "phase": "Succeeded",
                "startedAt": "2019-12-09T09:12:52Z",
                "templateName": "hello-hello-hello",
                "type": "StepGroup"
            },
            "steps-7lxxs-43875568": {
                "boundaryID": "steps-7lxxs",
                "children": [
                    "steps-7lxxs-1263033216"
                ],
                "displayName": "[1]",
                "finishedAt": "2019-12-09T09:12:52Z",
                "id": "steps-7lxxs-43875568",
                "name": "steps-7lxxs[1]",
                "phase": "Succeeded",
                "startedAt": "2019-12-09T09:12:44Z",
                "templateName": "hello-hello-hello",
                "type": "StepGroup"
            }
        },
        "phase": "Succeeded",
        "startedAt": "2019-12-09T09:12:37Z",
        "storedTemplates": {
            "/hello-hello-hello": {
                "arguments": {},
                "inputs": {},
                "metadata": {},
                "name": "hello-hello-hello",
                "outputs": {},
                "steps": [
                    [
                        {
                            "arguments": {
                                "parameters": [
                                    {
                                        "name": "message",
                                        "value": "hello1"
                                    }
                                ]
                            },
                            "name": "hello1",
                            "template": "whalesay"
                        }
                    ],
                    [
                        {
                            "arguments": {
                                "parameters": [
                                    {
                                        "name": "message",
                                        "value": "hello2"
                                    }
                                ]
                            },
                            "name": "hello2",
                            "template": "whalesay"
                        }
                    ],
                    [
                        {
                            "arguments": {
                                "parameters": [
                                    {
                                        "name": "message",
                                        "value": "hello1"
                                    }
                                ]
                            },
                            "name": "hello3",
                            "template": "whalesay"
                        }
                    ]
                ]
            },
            "/whalesay": {
                "arguments": {},
                "container": {
                    "args": [
                        "hello1"
                    ],
                    "command": [
                        "cowsay"
                    ],
                    "image": "docker/whalesay",
                    "name": "",
                    "resources": {}
                },
                "inputs": {},
                "metadata": {},
                "name": "whalesay",
                "outputs": {}
            }
        }
    }
}
`
)

func TestUnmarshal(t *testing.T) {
	a := assert.New(t)
	output := []byte(testWorkflowDescription)
	wf, err := parseWorkflowResource(output)
	a.NoError(err)
	expectedNodes := []string{
		"steps-7lxxs-1184503397", "steps-7lxxs-1263033216", "steps-7lxxs-1288663778",
		"steps-7lxxs-2267726410", "steps-7lxxs-43331115", "steps-7lxxs-43875568", "steps-7lxxs"}
	a.Equal(len(expectedNodes), len(wf.Status.Nodes))
	for _, name := range expectedNodes {
		_, ok := wf.Status.Nodes[name]
		a.True(ok)
	}

	a.Equal(wf.Status.Phase, wfv1.NodePhase("Succeeded"))
}

func TestGetStepGroup(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	output := []byte(testWorkflowDescription)
	wf, err := parseWorkflowResource(output)
	a.NoError(err)

	stepGroupNames := []string{
		"",
		"steps-7lxxs-1184503397",
		"steps-7lxxs-43875568",
		"steps-7lxxs-43331115"}
	for i := 0; i < len(stepGroupNames)-1; i++ {
		stepGroupName, err := getStepGroup(wf, "steps-7lxxs", stepGroupNames[i])
		a.NoError(err)
		if stepGroupNames[i] == "" {
			a.Equal(stepGroupNames[i+1], stepGroupName)
		} else {
			a.Equal(stepGroupNames[i], stepGroupName)
		}
	}
}

func TestGetNextStepGroup(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	output := []byte(testWorkflowDescription)
	wf, err := parseWorkflowResource(output)
	a.NoError(err)

	stepGroupNames := []string{
		"steps-7lxxs-1184503397",
		"steps-7lxxs-43875568",
		"steps-7lxxs-43331115",
		""}
	for i := 0; i < len(stepGroupNames)-1; i++ {
		next, err := getNextStepGroup(wf, stepGroupNames[i])
		a.NoError(err)
		a.Equal(stepGroupNames[i+1], next)
	}
}

func TestGetPodNameByStepGroup(t *testing.T) {
	if os.Getenv("SQLFLOW_TEST") != "workflow" {
		t.Skip("argo: skip workflow tests")
	}
	a := assert.New(t)
	output := []byte(testWorkflowDescription)
	wf, err := parseWorkflowResource(output)
	a.NoError(err)

	stepGroupNames := []string{
		"steps-7lxxs-1184503397",
		"steps-7lxxs-43875568",
		"steps-7lxxs-43331115"}
	podNames := []string{
		"steps-7lxxs-2267726410",
		"steps-7lxxs-1263033216",
		"steps-7lxxs-1288663778"}
	for i := 0; i < len(stepGroupNames); i++ {
		podName, err := getPodNameByStepGroup(wf, stepGroupNames[i])
		a.NoError(err)
		a.Equal(podNames[i], podName)
	}
}
