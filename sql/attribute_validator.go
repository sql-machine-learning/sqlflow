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
	"strings"

	"github.com/asaskevich/govalidator"
)

var attributeRegions = map[string]string{
	"model":  "",
	"train":  "",
	"eval":   "",
	"pred":   "",
	"engine": "",
}

// ValidateAttributes validates the attributes are leagal.
func ValidateAttributes(attrs map[string]*attribute) error {
	for k, v := range attrs {
		keyParts := strings.Split(k, ".")
		if len(keyParts) != 2 {
			return fmt.Errorf("Attributes should have only 2 parts, got %d in %s", len(keyParts), k)
		}
		keyRegion := keyParts[0]
		if _, ok := attributeRegions[keyRegion]; !ok {
			return fmt.Errorf("Unsupported WITH attribute region: %s", keyRegion)
		}
		// test the value is of supported type:
		// string, int, float, []int, []float
		if stringAttr, ok := v.Value.(string); ok {
			if govalidator.IsInt(stringAttr) {
				return nil
			} else if govalidator.IsFloat(stringAttr) {
				return nil
			} else if govalidator.IsJSON(stringAttr) {
				// test if json object like [1,2,3]
				return nil
			} else if govalidator.IsASCII(stringAttr) {
				// NOTE: should avoid this checking is ascii
				return nil
			} else {
				return fmt.Errorf("Not supported attribute value: %s", stringAttr)
			}
		} else {
			return fmt.Errorf("Attribute value is not string type, %v", v.Value)
		}
	}
	return nil
}
