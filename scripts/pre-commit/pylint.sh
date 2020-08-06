#!/bin/bash
# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

if [[ "$TRAVIS_BUILD_DIR" != "" ]]; then
    # CI should check all files in ./python
    file_or_dir_to_check=$TRAVIS_BUILD_DIR/python
else
    # Local pre-commit would check the changed files only
    file_or_dir_to_check=$(git diff --cached --name-only --diff-filter=ACMR | grep '\.py$' )
fi

if [[ "$file_or_dir_to_check" == "" ]]; then
    exit 0
fi

# TODO(sneaxiy): enable pylint on CI after fixing so many errors
if [[ "$TRAVIS_BUILD_DIR" == "" ]]; then
    pylint "$file_or_dir_to_check"
fi

flake8 "$file_or_dir_to_check"
