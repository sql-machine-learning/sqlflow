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


# NOTE(sneaxiy): model.save(...) would save the weights in the directory
# ./variables, which is in the format of TensorFlow checkpoint.
# model.save_weights(...) also saves the weights in the format of
# TensorFlow checkpoint. So we can load the weights which are saved
# using model.save(...) method.
def load_keras_model_weights(model, path):
    return model.load_weights(os.path.join(path, "variables/variables"))
