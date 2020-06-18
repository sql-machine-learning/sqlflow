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

import copy
import json


def collect_model_metadata(select, validate_select, estimator, attributes,
                           feature_columns, field_descs, label, evaluation,
                           model_repo_image):
    """ collect kinds of model metadata and put them in a dict """
    metadata = dict(locals())
    attr_copy = copy.deepcopy(attributes)
    for (k, v) in attr_copy.items():
        try:
            json.dumps(v)
        except:
            attr_copy[k] = str(v)
    metadata['attributes'] = attr_copy
    return metadata


def save_model_metadata(path, metadata):
    """save_model_metdata saves given params into 'path'"""
    with open(path, mode="w") as meta_file:
        meta_file.write(json.dumps(metadata, indent=2))


def load_model_metadata(path):
    """load_model_metadata load metadata from given 'path'"""
    with open(path, mode="r") as meta_file:
        lines = meta_file.readlines()
        return json.loads(lines)
