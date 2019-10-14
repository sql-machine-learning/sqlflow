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
	"strconv"
	"strings"
)

type attribute struct {
	FullName string
	Prefix   string
	Name     string
	Value    interface{}
}

type gitLabModule struct {
	ModuleName   string
	ProjectName  string
	Sha          string
	PrivateToken string
	SourceRoot   string
	GitLabServer string
}

type engineSpec struct {
	etype                 string
	ps                    resourceSpec
	worker                resourceSpec
	cluster               string
	queue                 string
	masterResourceRequest string
	masterResourceLimit   string
	workerResourceRequest string
	workerResourceLimit   string
	volume                string
	imagePullPolicy       string
	restartPolicy         string
	extraPypiIndex        string
	namespace             string
	minibatchSize         int
	masterPodPriority     string
	clusterSpec           string
	numMinibatchesPerTask int
	dockerImageRepository string
	envs                  string
}

func getEngineSpec(attrs map[string]*attribute) engineSpec {
	getInt := func(key string, defaultValue int) int {
		if p, ok := attrs[key]; ok {
			strVal, _ := p.Value.(string)
			intVal, err := strconv.Atoi(strVal)

			if err == nil {
				return intVal
			}
		}
		return defaultValue
	}
	getString := func(key string, defaultValue string) string {
		if p, ok := attrs[key]; ok {
			strVal, ok := p.Value.(string)
			if ok {
				// TODO(joyyoj): use the parser to do those validations.
				if strings.HasPrefix(strVal, "\"") && strings.HasSuffix(strVal, "\"") {
					return strVal[1 : len(strVal)-1]
				}
				return strVal
			}
		}
		return defaultValue
	}

	psNum := getInt("ps_num", 1)
	psMemory := getInt("ps_memory", 2400)
	workerMemory := getInt("worker_memory", 1600)
	workerNum := getInt("worker_num", 2)
	engineType := getString("type", "local")
	if (psNum > 0 || workerNum > 0) && engineType == "local" {
		engineType = "yarn"
	}
	cluster := getString("cluster", "")
	queue := getString("queue", "")

	// ElasticDL engine specs
	masterResourceRequest := getString("master_resource_request", "cpu=0.1,memory=1024Mi")
	masterResourceLimit := getString("master_resource_limit", "")
	workerResourceRequest := getString("worker_resource_request", "cpu=1,memory=4096Mi")
	workerResourceLimit := getString("worker_resource_limit", "")
	volume := getString("volume", "")
	imagePullPolicy := getString("image_pull_policy", "Always")
	restartPolicy := getString("restart_policy", "Never")
	extraPypiIndex := getString("extra_pypi_index", "")
	namespace := getString("namespace", "default")
	minibatchSize := getInt("minibatch_size", 64)
	masterPodPriority := getString("master_pod_priority", "")
	clusterSpec := getString("cluster_spec", "")
	numMinibatchesPerTask := getInt("num_minibatches_per_task", 10)
	dockerImageRepository := getString("docker_image_repository", "")
	envs := getString("envs", "")

	return engineSpec{
		etype:                 engineType,
		ps:                    resourceSpec{Num: psNum, Memory: psMemory},
		worker:                resourceSpec{Num: workerNum, Memory: workerMemory},
		cluster:               cluster,
		queue:                 queue,
		masterResourceRequest: masterResourceRequest,
		masterResourceLimit:   masterResourceLimit,
		workerResourceRequest: workerResourceRequest,
		workerResourceLimit:   workerResourceLimit,
		volume:                volume,
		imagePullPolicy:       imagePullPolicy,
		restartPolicy:         restartPolicy,
		extraPypiIndex:        extraPypiIndex,
		namespace:             namespace,
		minibatchSize:         minibatchSize,
		masterPodPriority:     masterPodPriority,
		clusterSpec:           clusterSpec,
		numMinibatchesPerTask: numMinibatchesPerTask,
		dockerImageRepository: dockerImageRepository,
		envs:                  envs,
	}
}

func (a *attribute) GenerateCode() (string, error) {
	if val, ok := a.Value.(string); ok {
		// First try converting a hyperparameter to int.
		if _, err := strconv.Atoi(val); err == nil {
			return fmt.Sprintf("%s=%s", a.Name, val), nil
		}
		// Then try converting the hyperparameter to float.
		if _, err := strconv.ParseFloat(val, 32); err == nil {
			return fmt.Sprintf("%s=%s", a.Name, val), nil
		}
		// Hyperparameters of other types generate quoted plain string.
		return fmt.Sprintf("%s=\"%s\"", a.Name, val), nil
	}
	if val, ok := a.Value.([]interface{}); ok {
		intList, err := transformToIntList(val)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s=%s", a.Name,
			strings.Join(strings.Split(fmt.Sprint(intList), " "), ",")), nil
	}
	return "", fmt.Errorf("the value type of an attribute must be string, int, float or list of ints, given %s", a.Value)
}

func attrFilter(attrs map[string]*attribute, prefix string, remove bool) map[string]*attribute {
	ret := make(map[string]*attribute, 0)
	for _, a := range attrs {
		if strings.EqualFold(a.Prefix, prefix) {
			ret[a.Name] = a
			if remove {
				delete(attrs, a.FullName)
			}
		}
	}
	return ret
}
