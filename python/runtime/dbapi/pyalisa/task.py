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
# limitations under the License

import base64
import json
from collections import OrderedDict

from six.moves.urllib.parse import parse_qs, urlparse


class Task(object):
    """ run alisa task synchronously: submit & wait a task
    """
    @staticmethod
    def exec_sql():
        pass


    @staticmethod
    def exec_pyodps():
        pass


