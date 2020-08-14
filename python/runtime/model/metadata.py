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

from runtime.feature.column import (JSONDecoderWithFeatureColumn,
                                    JSONEncoderWithFeatureColumn)


def collect_metadata(original_sql,
                     select,
                     validation_select,
                     model_repo_image,
                     class_name,
                     attributes,
                     features=None,
                     label=None,
                     evaluation=None,
                     **kwargs):
    """
    Collect kinds of model metadata and put them in a dict. The parameter list
    of this method is almost the same as the ir.TrainStmt in the Go side.

    Args:
        original_sql (str): the original SQL statement.
        select (str): the select statement.
        validation_select (str): the validation select statement.
        model_repo_image (str): the model repo docker image name.
        class_name (str): the estimator name.
        attributes (dict): the attribute map.
        features (dict[str->list[FeatureColumn]]): the feature column map.
        label (FeatureColumn): the label column.
        evaluation (dict): the evaluation result of the model.
        kwargs (dict): any extra metadata to be saved.

    Returns:
        A dict of metadata to be saved.
    """
    metadata = dict(locals())

    kwargs = metadata.pop('kwargs')
    if kwargs:
        metadata.update(kwargs)

    attr_copy = copy.deepcopy(attributes)
    for (k, v) in attr_copy.items():
        try:
            json.dumps(v)
        except:  # noqa: E722
            attr_copy[k] = str(v)
    metadata['attributes'] = attr_copy
    return metadata


def save_metadata(path, metadata):
    """
    Save given metadata into the local 'path'.
    """
    with open(path, mode="w") as meta_file:
        meta_file.write(
            json.dumps(metadata, indent=2, cls=JSONEncoderWithFeatureColumn))


def load_metadata(path):
    """
    Load the metadata from the given local 'path'.
    """
    with open(path, mode="r") as meta_file:
        content = meta_file.read()

    return json.loads(content, cls=JSONDecoderWithFeatureColumn)
