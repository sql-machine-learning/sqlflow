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

package test

import (
	"container/list"
	"fmt"
	"os"
	"strings"
)

// GetEnv returns the environment variable value or the given default value
func GetEnv(env, value string) string {
	if v := os.Getenv(env); len(v) > 0 {
		return v
	}
	return value
}

// GoodStream checks whether there's errors in `stream`
func GoodStream(stream chan interface{}) (bool, string) {
	lastResp := list.New()
	keepSize := 10

	for rsp := range stream {
		switch rsp.(type) {
		case error:
			var ss []string
			for e := lastResp.Front(); e != nil; e = e.Next() {
				if s, ok := e.Value.(string); ok {
					ss = append(ss, s)
				}
			}
			return false, fmt.Sprintf("%v: %s", rsp, strings.Join(ss, "\n"))
		}
		lastResp.PushBack(rsp)
		if lastResp.Len() > keepSize {
			e := lastResp.Front()
			lastResp.Remove(e)
		}
	}
	return true, ""
}
