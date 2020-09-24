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

package pai

import "fmt"

// PSConfig implicates Parameter Server Config
type PSConfig struct {
	Count int `json:"count"`
	GPU   int `json:"gpu"`
	CPU   int `json:"cpu"`
}

// WorkerConfig implicates Worker Config
type WorkerConfig struct {
	Count int `json:"count"`
	GPU   int `json:"gpu"`
	CPU   int `json:"cpu"`
}

// ClusterConfig implicates PAI distributed task meta
type ClusterConfig struct {
	PS        PSConfig      `json:"ps"`
	Worker    WorkerConfig  `json:"worker"`
	Evaluator *WorkerConfig `json:"evaluator,omitempty"`
}

// GetClusterConfig returns ClusterConfig object comes from WITH clause
func GetClusterConfig(attrs map[string]interface{}) (*ClusterConfig, error) {
	defaultMap := map[string]int{
		"train.num_ps":        0,
		"train.num_workers":   1,
		"train.worker_cpu":    400,
		"train.worker_gpu":    0,
		"train.ps_cpu":        200,
		"train.ps_gpu":        0,
		"train.num_evaluator": 0,
		"train.evaluator_cpu": 200,
		"train.evaluator_gpu": 0,
	}
	for k := range defaultMap {
		attrValue, ok := attrs[k]
		if ok {
			intValue, intok := attrValue.(int)
			if !intok {
				return nil, fmt.Errorf("attribute %s must be int, got: %s", k, attrValue)
			}
			defaultMap[k] = intValue
			delete(attrs, k)
		}
	}
	cc := &ClusterConfig{
		PS: PSConfig{
			Count: defaultMap["train.num_ps"],
			CPU:   defaultMap["train.ps_cpu"],
			GPU:   defaultMap["train.ps_gpu"],
		},
		Worker: WorkerConfig{
			Count: defaultMap["train.num_workers"],
			CPU:   defaultMap["train.worker_cpu"],
			GPU:   defaultMap["train.worker_gpu"],
		},
	}
	// FIXME(weiguoz): adhoc for running distributed xgboost train on pai
	if cc.Worker.Count > 1 && cc.PS.Count < 1 {
		cc.PS.Count = 1
	}

	if defaultMap["train.num_evaluator"] == 0 {
		cc.Evaluator = nil
	} else if defaultMap["train.num_evaluator"] == 1 {
		cc.Evaluator = &WorkerConfig{
			Count: defaultMap["train.num_evaluator"],
			CPU:   defaultMap["train.evaluator_cpu"],
			GPU:   defaultMap["train.evaluator_gpu"],
		}
	} else if defaultMap["train.num_evaluator"] > 1 {
		return nil, fmt.Errorf("train.num_evaluator should only be 1 or 0")
	}
	return cc, nil
}

// GetClusterConfig4Pred returns ClusterConfig object comes from WITH clause
func GetClusterConfig4Pred(attrs map[string]interface{}) (*ClusterConfig, error) {
	defaultMap := map[string]int{
		"predict.num_workers": 1,
	}
	for k := range defaultMap {
		attrValue, ok := attrs[k]
		if ok {
			intValue, intok := attrValue.(int)
			if !intok {
				return nil, fmt.Errorf("attribute %s must be int, got: %s", k, attrValue)
			}
			defaultMap[k] = intValue
			delete(attrs, k)
		}
	}
	cc := &ClusterConfig{
		PS: PSConfig{
			Count: 1,
		},
		Worker: WorkerConfig{
			Count: defaultMap["predict.num_workers"],
		},
	}
	if cc.Worker.Count < 1 {
		return nil, fmt.Errorf("invalid predict.num_workers")
	}
	return cc, nil
}
