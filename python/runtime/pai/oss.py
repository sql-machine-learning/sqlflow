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

import oss2


def copyfileobj(source, dest, ak, sk, endpoint, bucket_name):
    '''
    copy_file_to_oss copies (`source`(local file) to an object on OSS
    '''
    auth = oss2.Auth(ak, sk)
    bucket = oss2.Bucket(auth, endpoint, bucket_name)
    # overwrite if exists
    bucket.put_object_from_file(dest, source)


def get_bucket(name, ak=None, sk=None, endpoint=None):
    if ak is None:
        ak = os.getenv("SQLFLOW_OSS_AK", "")

    if sk is None:
        sk = os.getenv("SQLFLOW_OSS_SK", "")

    if endpoint is None:
        endpoint = os.getenv("SQLFLOW_OSS_MODEL_ENDPOINT", "")

    if ak == "" or sk == "":
        raise ValueError(
            "must configure SQLFLOW_OSS_AK and SQLFLOW_OSS_SK when submitting to PAI"
        )

    if endpoint == "":
        raise ValueError(
            "must configure SQLFLOW_OSS_MODEL_ENDPOINT when submitting to PAI")

    auth = oss2.Auth(ak, sk)
    bucket = oss2.Bucket(auth, endpoint, name)
    return bucket
