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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

const getClassesCode string = `import sys, inspect, json
def print_classes(package_name):
    pkg_module = __import__(package_name)
    for name, obj in inspect.getmembers(pkg_module):
        if inspect.isclass(obj):
            docstring = obj.__init__.__doc__
            if docstring == None:
                docstring = ""
            print(json.dumps({"name": obj.__name__, "docstring": docstring}))
print_classes("%s")`

type modelClassDesc struct {
	Name      string `json:"name"`
	Type      int    `json:"type,omitempty"` // type should be 0 (class) or 1 (function)
	DocString string `json:"docstring"`
}

func getModelClasses(dir string) ([]*modelClassDesc, error) {
	// get sub directories as the model packages.
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pythonPackages := []string{}
	dockerfileFound := false
	for _, f := range files {
		if f.IsDir() {
			pythonPackages = append(pythonPackages, f.Name())
		} else {
			if f.Name() == "Dockerfile" {
				dockerfileFound = true
			}
		}
	}
	if !dockerfileFound {
		return nil, fmt.Errorf("releasing a model definition requires a Dockerfile under your repo directory")
	}
	if len(pythonPackages) == 0 {
		return nil, fmt.Errorf("folder %s got no sub-folders as Python packages", dir)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err := os.Chdir(dir); err != nil {
		return nil, err
	}

	result := []*modelClassDesc{}
	for _, pname := range pythonPackages {
		cmd := exec.Command("python")
		cmd.Stdin = bytes.NewBufferString(fmt.Sprintf(getClassesCode, pname))
		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		// filter empty new lines
		for _, line := range strings.Split(string(out), "\n") {
			if line != "" {
				desc := &modelClassDesc{Name: "", Type: 0, DocString: ""}
				err := json.Unmarshal([]byte(line), desc)
				if err != nil {
					return nil, err
				}
				result = append(result, desc)
			}
		}
	}

	if err := os.Chdir(cwd); err != nil {
		return nil, err
	}
	return result, nil
}
