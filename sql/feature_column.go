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

// featureColumn is an interface that all types of feature columns and
// attributes (WITH clause) should follow.
// featureColumn is used to generate feature column code.
type featureColumn interface {
	GenerateCode() (string, error)
	// Some feature columns accept input tensors directly, and the data
	// may be a tensor string like: 12,32,4,58,0,0
	GetDelimiter() string
	GetDtype() string
	GetKey() string
	// return input shape json string, like "[2,3]"
	GetInputShape() string
}
