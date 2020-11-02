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


class Config(object):
    """Alisa config object, this can be parsed from an alisa dsn

    Args:
        url(string): a connection url like :
        "alisa://user:pwd@host/path?env=AAB&with=XSE".
        There are three required params in the url: current_project,
        env and with. The env and with params are maps, which is
        dumpped to json and then encoded in base64 format, that is:
        env=base64(json.dumps({"a":1, "b":2}))
    """
    def __init__(self, url=None):
        if url:
            self._parse_url(url)

    def _parse_url(self, url):
        urlpts = urlparse(url)
        kvs = parse_qs(urlpts.query)
        required = ["env", "with", "curr_project"]
        for k in required:
            if k not in kvs:
                raise ValueError("Given dsn does not contain: %s" % k)
        # extract the param if it's only has one element
        for k, v in kvs.items():
            if len(v) == 1:
                kvs[k] = v[0]

        self.pop_access_id = urlpts.username
        self.pop_access_secret = urlpts.password
        self.pop_url = urlpts.hostname + urlpts.path
        self.pop_scheme = urlpts.scheme

        self.env = Config._decode_json_base64(kvs["env"])
        self.withs = Config._decode_json_base64(kvs["with"])
        self.scheme = kvs["scheme"] or "http"
        self.verbose = kvs["verbose"] == "true"
        self.curr_project = kvs["curr_project"]

    @staticmethod
    def _encode_json_base64(env):
        # We sort the env params to ensure the consistent encoding
        jstr = json.dumps(OrderedDict(env))
        b64 = base64.urlsafe_b64encode(jstr.encode("utf8")).decode("utf8")
        return b64.rstrip("=")

    @staticmethod
    def _decode_json_base64(b64env):
        padded = b64env + "=" * (len(b64env) % 4)
        jstr = base64.urlsafe_b64decode(padded).decode("utf8")
        return json.loads(jstr)

    def to_url(self):
        """Serialize a config to connection url

        Returns:
            (string) a connection url build from this config
        """
        parts = (
            self.pop_access_id,
            self.pop_access_secret,
            self.pop_url,
            self.scheme,
            "true" if self.verbose else "false",
            self.curr_project,
            Config._encode_json_base64(self.env),
            Config._encode_json_base64(self.withs),
        )
        return ("alisa://%s:%s@%s?scheme=%s&verbose"
                "=%s&curr_project=%s&env=%s&with=%s") % parts
