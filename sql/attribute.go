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

func (a *attribute) GenerateCode() (string, error) {
	if val, ok := a.Value.(string); ok {
		// auto convert to int first.
		if _, err := strconv.Atoi(val); err == nil {
			return fmt.Sprintf("%s=%s", a.Name, val), nil
		}
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
	return "", fmt.Errorf("value of attribute must be string or list of int, given %s", a.Value)
}

func filter(attrs map[string]*attribute, prefix string, remove bool) map[string]*attribute {
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

func resolveAttribute(attrs *attrs) (map[string]*attribute, error) {
	ret := make(map[string]*attribute)
	for k, v := range *attrs {
		subs := strings.SplitN(k, ".", 2)
		name := subs[len(subs)-1]
		prefix := ""
		if len(subs) == 2 {
			prefix = subs[0]
		}
		r, err := resolveExpression(v)
		if err != nil {
			return nil, err
		}

		a := &attribute{
			FullName: k,
			Prefix:   prefix,
			Name:     name,
			Value:    r}
		ret[a.FullName] = a
	}
	return ret, nil
}
