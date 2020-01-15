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
import uuid
from contextlib import contextmanager

import oss2

oss_internal_endpoints = {
    # From https://help.aliyun.com/document_detail/31837.html?spm=a2c4g.11186623.2.20.3eba7f5eufj1Pt#concept-zt4-cvy-5db
    "oss-ap-northeast-1.aliyuncs.com":
    "oss-ap-northeast-1-internal.aliyuncs.com",
    "oss-ap-south-1.aliyuncs.com": "oss-ap-south-1-internal.aliyuncs.com",
    "oss-ap-southeast-1.aliyuncs.com":
    "oss-ap-southeast-1-internal.aliyuncs.com",
    "oss-ap-southeast-2.aliyuncs.com":
    "oss-ap-southeast-2-internal.aliyuncs.com",
    "oss-ap-southeast-3.aliyuncs.com":
    "oss-ap-southeast-3-internal.aliyuncs.com",
    "oss-ap-southeast-5.aliyuncs.com":
    "oss-ap-southeast-5-internal.aliyuncs.com",
    "oss-cn-beijing.aliyuncs.com": "oss-cn-beijing-internal.aliyuncs.com",
    "oss-cn-chengdu.aliyuncs.com": "oss-cn-chengdu-internal.aliyuncs.com",
    "oss-cn-hangzhou.aliyuncs.com": "oss-cn-hangzhou-internal.aliyuncs.com",
    "oss-cn-hongkong.aliyuncs.com": "oss-cn-hongkong-internal.aliyuncs.com",
    "oss-cn-huhehaote.aliyuncs.com": "oss-cn-huhehaote-internal.aliyuncs.com",
    "oss-cn-qingdao.aliyuncs.com": "oss-cn-qingdao-internal.aliyuncs.com",
    "oss-cn-shanghai.aliyuncs.com": "oss-cn-shanghai-internal.aliyuncs.com",
    "oss-cn-shenzhen.aliyuncs.com": "oss-cn-shenzhen-internal.aliyuncs.com",
    "oss-cn-zhangjiakou.aliyuncs.com":
    "oss-cn-zhangjiakou-internal.aliyuncs.com",
    "oss-eu-central-1.aliyuncs.com": "oss-eu-central-1-internal.aliyuncs.com",
    "oss-eu-west-1.aliyuncs.com": "oss-eu-west-1-internal.aliyuncs.com",
    "oss-me-east-1.aliyuncs.com": "oss-me-east-1-internal.aliyuncs.com",
    "oss-us-east-1.aliyuncs.com": "oss-us-east-1-internal.aliyuncs.com",
    "oss-us-west-1.aliyuncs.com": "oss-us-west-1-internal.aliyuncs.com",
}


@contextmanager
def oss_temporary(ak, sk, endpoint, filename):
    '''
    oss_temporary copies `filename` to a temporary object on OSS and deletes it a.s.a.p.
    Example: 
    with oss_temporary(YOUR_AK, YOUR_SK, ENDPOINT, 'test.py') as f:
        do_something_with(f)
    '''
    bucket_name = 'sqlflow-pai-submitter'
    auth = oss2.Auth(ak, sk)
    bucket = oss2.Bucket(auth, endpoint, bucket_name)

    bucket.create_bucket(
        oss2.BUCKET_ACL_PRIVATE,
        oss2.models.BucketCreateConfig(oss2.BUCKET_STORAGE_CLASS_IA))

    name = uuid.uuid4().hex
    if bucket.object_exists(name):
        raise FileExistsError("[Errno 17] File exists: '%s'" %
                              name)  # This would never happen.
    else:
        bucket.put_object_from_file(name, filename)
    yield f'oss://{bucket_name}.{internal_endpoints[endpoint]}/{name}'
    bucket.delete_object(name)
