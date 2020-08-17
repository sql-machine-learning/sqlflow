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

from runtime.diagnostics import SQLFlowDiagnostic


def get_cluster_config(attrs):
    """Get PAI cluster config from attrs

    Args:
        attrs: input config

    Returns:
        The merged config by attrs and default
    """
    default_map = {
        "train.num_ps": 0,
        "train.num_workers": 1,
        "train.worker_cpu": 400,
        "train.worker_gpu": 0,
        "train.ps_cpu": 200,
        "train.ps_gpu": 0,
        "train.num_evaluator": 0,
        "train.evaluator_cpu": 200,
        "train.evaluator_gpu": 0,
    }
    update = dict([(k, v) for (k, v) in attrs.items() if k in default_map])
    if not all(isinstance(v, int) for v in update.values()):
        raise SQLFlowDiagnostic("value for cluster config should be int")
    default_map.update(attrs)

    ps = {
        "count": default_map["train.num_ps"],
        "cpu": default_map["train.ps_cpu"],
        "gpu": default_map["train.ps_gpu"],
    }
    worker = {
        "count": default_map["train.num_workers"],
        "cpu": default_map["train.worker_cpu"],
        "gpu": default_map["train.worker_gpu"],
    }
    # FIXME(weiguoz): adhoc for running distributed xgboost train on pai
    if worker["count"] > 1 and ps["count"] < 1:
        ps["count"] = 1

    if default_map["train.num_evaluator"] == 0:
        evaluator = None
    elif default_map["train.num_evaluator"] == 1:
        evaluator = {
            "count": default_map["train.num_evaluator"],
            "cpu": default_map["train.evaluator_cpu"],
            "gpu": default_map["train.evaluator_gpu"],
        }
    else:
        raise SQLFlowDiagnostic("train.num_evaluator should only be 1 or 0")
    conf = {"ps": ps, "worker": worker}
    if evaluator is not None:
        conf["evaluator"] = evaluator
    return conf
