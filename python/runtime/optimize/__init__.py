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

from runtime.optimize.optflow import run_optimize_on_optflow  # noqa: F401

# Step images which submit job to OptFlow may not require pyomo module
try:
    from runtime.optimize.local import generate_model_with_data_frame  # noqa: F401, E501
    from runtime.optimize.local import run_optimize_locally  # noqa: F401, E501
except:  # noqa: E722
    pass
