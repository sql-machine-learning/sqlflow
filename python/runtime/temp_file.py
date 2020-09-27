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

import os
import shutil
import tempfile


# NOTE: Python 2 does not have tempfile.TemporaryDirectory. To unify the code
# of Python 2 and 3, we make the following class.
class TemporaryDirectory(object):
    def __init__(self, as_cwd=False, suffix=None, prefix=None, dir=None):
        """
        Create a temporary directory.

        Args:
            as_cwd (bool): whether to change the current working directory
                as the created temporary directory.
            suffix (str): the suffix of the created temporary directory.
            prefix (str): the prefix of the created temporary directory.
            dir (str): where to create the temporary directory.
        """
        if suffix is None:
            suffix = ""

        if prefix is None:
            prefix = ""

        if dir is None:
            dir = "/tmp"

        self.tmp_dir = tempfile.mkdtemp(suffix=suffix, prefix=prefix, dir=dir)
        self.as_cwd = as_cwd
        if self.as_cwd:
            self.old_dir = os.getcwd()

    def __enter__(self, *args, **kwargs):
        if self.as_cwd:
            os.chdir(self.tmp_dir)
        return self.tmp_dir

    def __exit__(self, *args, **kwargs):
        if self.as_cwd:
            os.chdir(self.old_dir)
        shutil.rmtree(self.tmp_dir, ignore_errors=True)
