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
	"fmt"
	"regexp"
)

// checkName checks model definition and trained model is valid.
func checkName(name string) error {
	if len(name) < 6 {
		return fmt.Errorf("model name should have at least 6 characters")
	}
	match, err := regexp.MatchString(`^[a-zA-Z0-9_\-\.]{6,256}$`, name)
	if err != nil {
		return err
	}
	if !match {
		return fmt.Errorf("model name should be constist of letters, numbers, underscroll, dash, and must start with a letter")
	}
	return nil
}

// checkTag checks tag is valid. Tags should be consist of alphabet, numbers, dashes (-), underscrolls (_) and dots (.).
func checkTag(name string) error {
	match, err := regexp.MatchString(`^[a-zA-Z0-9\_\-\.]{0,256}$`, name)
	if err != nil {
		return err
	}
	if !match {
		return fmt.Errorf("model tag should be constist of letters, numbers, underscroll, dash, and dots")
	}
	return nil
}

// checkImageURL checks a Docker image URL is valid
func checkImageURL(imageURL string) error {
	if len(imageURL) < 6 {
		return fmt.Errorf("imageURL should have at least 6 characters")
	}

	match, err := regexp.MatchString(`^[a-zA-Z0-9\._\/\-]{6,256}$`, imageURL)
	if err != nil {
		return err
	}

	if !match {
		return fmt.Errorf("docker image URL should be format like [hub.your-registry.com/group/]your_image_name")
	}
	return nil
}

func checkNameAndTag(name, tag string) error {
	if err := checkName(name); err != nil {
		return err
	}
	if err := checkTag(tag); err != nil {
		return err
	}
	return nil
}

func checkImageAndTag(imageURL, tag string) error {
	if err := checkImageURL(imageURL); err != nil {
		return err
	}
	if err := checkTag(tag); err != nil {
		return err
	}
	return nil
}
