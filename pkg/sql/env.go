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

package sql

import (
	"fmt"
	"os"
	"strconv"
)

// SetTensorFlowRandomSeed set the tensorflow random seed in environment variable
// It is useful when we want to get deterministic results during training, for
// example, deterministic results in unittests, reproducible model performance, etc.
func SetTensorFlowRandomSeed(seed interface{}) error {
	envKey := "SQLFLOW_TF_RANDOM_SEED"
	if seed == nil {
		return os.Unsetenv(envKey)
	}

	if s, ok := seed.(int); ok {
		return os.Setenv(envKey, strconv.Itoa(s))
	}

	return fmt.Errorf("unsupported seed type, must be int or nil, but got %T", seed)
}
