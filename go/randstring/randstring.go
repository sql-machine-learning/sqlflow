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

package randstring

import "math/rand"

// Generate returns a random string containing letters and digits of length n.
func Generate(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const lettersAndDigits = letters + "0123456789"
	b := make([]byte, n)
	// do not start from digit
	b[0] = letters[rand.Intn(len(letters))]
	for i := 1; i < len(b); i++ {
		b[i] = lettersAndDigits[rand.Intn(len(lettersAndDigits))]
	}
	return string(b)
}
