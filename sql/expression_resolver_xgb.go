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
)

type resolvedXGBTrainClause struct {
	NumBoostRound int
	Maximize      bool
	ParamsAttr    map[string]*attribute
}

func resolveXGBTrainClause(tc *trainClause) (*resolvedXGBTrainClause, error) {
	attrs, err := resolveAttribute(&tc.trainAttrs)
	if err != nil {
		return nil, err
	}
	getIntAttr := func(key string, defaultValue int) int {
		if p, ok := attrs[key]; ok {
			strVal, _ := p.Value.(string)
			intVal, err := strconv.Atoi(trimQuotes(strVal))
			defer delete(attrs, p.FullName)
			if err == nil {
				return intVal
			}
			fmt.Printf("ignore invalid %s=%s, default is %d", key, p.Value, defaultValue)
		}
		return defaultValue
	}
	getBoolAttr := func(key string, defaultValue bool, optional bool) bool {
		if p, ok := attrs[key]; ok {
			strVal, _ := p.Value.(string)
			boolVal, err := strconv.ParseBool(trimQuotes(strVal))
			if !optional {
				defer delete(attrs, p.FullName)
			}
			if err == nil {
				return boolVal
			} else if !optional {
				fmt.Printf("ignore invalid %s=%s, default is %v", key, p.Value, defaultValue)
			}
		}
		return defaultValue
	}
	return &resolvedXGBTrainClause{
		NumBoostRound: getIntAttr("train.num_boost_round", 10),
		Maximize:      getBoolAttr("train.maximize", false, true),
		ParamsAttr:    filter(attrs, "params", true),
	}, nil
}
