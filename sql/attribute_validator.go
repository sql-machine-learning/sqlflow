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
	"model":   "",
	"train":   "",
	"eval":    "",
	"predict": "",
	"engine":  "",
}

// ValidateAttributes validates the attributes are legal.
func ValidateAttributes(attrs map[string]*attribute) error {
	for k, v := range attrs {
		keyParts := strings.Split(k, ".")
		if len(keyParts) != 2 {
			return fmt.Errorf("attributes should of format first_part.second_part, got %s", k)
		}
		keyRegion := keyParts[0]
		if _, ok := attributeRegions[keyRegion]; !ok {
			return fmt.Errorf("unsupported WITH attribute region: %s", keyRegion)
		}
		// test the value is of supported type:
		// string, int, float, []int
		switch value := v.Value.(type) {
		case string:
			if govalidator.IsInt(value) || govalidator.IsFloat(value) || govalidator.IsJSON(value) || govalidator.IsASCII(value) {
				continue
			}
			return fmt.Errorf("not supported attribute value: %s", value)
		case []int, []interface{}:
			continue
		default:
			return fmt.Errorf("unrecognize attribute %v with type %T", value, value)
		}
	}
	return nil
}
