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

package alps

import (
	"strconv"
	"strings"
)

type gitLabModule struct {
	ModuleName   string
	ProjectName  string
	Sha          string
	PrivateToken string
	SourceRoot   string
	GitLabServer string
}

type resourceSpec struct {
	Num    int
	Memory int
	Core   int
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
	jobName               string
	imageBase             string
}

func getEngineSpecWithIR(attrs map[string]interface{}) engineSpec {
	getInt := func(key string, defaultValue int) int {
		if p, ok := attrs[key]; ok {
			strVal, _ := p.(string)
			intVal, err := strconv.Atoi(strVal)

			if err == nil {
				return intVal
			}
		}
		return defaultValue
	}
	getString := func(key string, defaultValue string) string {
		if p, ok := attrs[key]; ok {
			strVal, ok := p.(string)
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
	jobName := getString("job_name", "")
	imageBase := getString("image_base", "")

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
		jobName:               jobName,
		imageBase:             imageBase,
	}
}
