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

import grpc
from runtime.feature.column import JSONDecoderWithFeatureColumn
from runtime.model.modelzooserver_pb2 import ReleaseModelRequest
from runtime.model.modelzooserver_pb2_grpc import ModelZooServerStub


def load_model_from_model_zoo(address, table, tag, tarball_path):
    with grpc.insecure_channel(address) as channel:
        stub = ModelZooServerStub(channel)

        meta_req = ReleaseModelRequest(name=table, tag=tag)
        meta_resp = stub.GetModelMeta(meta_req)
        meta = json.loads(meta_resp.meta, cls=JSONDecoderWithFeatureColumn)

        tar_req = ReleaseModelRequest(name=table, tag=tag)
        tar_resp = stub.DownloadModel(tar_req)
        with open(tarball_path, "wb") as f:
            for each_resp in tar_resp:
                f.write(bytes(each_resp.content_tar))

    return meta
