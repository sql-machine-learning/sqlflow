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

import json
import sys

import grpc
import six
from runtime.feature.column import JSONDecoderWithFeatureColumn
from runtime.model.modelzooserver_pb2 import ReleaseModelRequest
from runtime.model.modelzooserver_pb2_grpc import ModelZooServerStub


def load_model_from_model_zoo(address, model, tag):
    stub = None
    meta = None
    channel = grpc.insecure_channel(address)
    try:
        stub = ModelZooServerStub(channel)
        meta_req = ReleaseModelRequest(name=model, tag=tag)
        meta_resp = stub.GetModelMeta(meta_req)
        meta = json.loads(meta_resp.meta, cls=JSONDecoderWithFeatureColumn)
    except:  # noqa: E722
        # make sure that the channel is closed when exception raises
        channel.close()
        six.reraise(*sys.exc_info())

    def reader():
        with channel:
            tar_req = ReleaseModelRequest(name=model, tag=tag)
            tar_resp = stub.DownloadModel(tar_req)
            for each_resp in tar_resp:
                yield each_resp.content_tar

    return reader, meta
