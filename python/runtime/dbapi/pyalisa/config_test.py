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

import unittest

from runtime.dbapi.pyalisa.config import Config

test_url = ("alisa://pid:psc@dw.a.hk/?scheme=http&verbose=true&"
            "curr_project=jtest_env&env=eyJTS1lORVRfT05EVVRZIjog"
            "IlNLWSIsICJTS1lORVRfQUNDRVNTSUQiOiAiU0tZIiwgIlNLWU5"
            "FVF9TWVNURU1JRCI6ICJTS1kiLCAiQUxJU0FfVEFTS19JRCI6IC"
            "JBTEkiLCAiU0tZTkVUX0VORFBPSU5UIjogIlNLWSIsICJTS1lOR"
            "VRfU1lTVEVNX0VOViI6ICJTS1kiLCAiU0tZTkVUX0JJWkRBVEUi"
            "OiAiU0tZIiwgIlNLWU5FVF9BQ0NFU1NLRVkiOiAiU0tZIiwgIlNL"
            "WU5FVF9QQUNLQUdFSUQiOiAiU0tZIiwgIkFMSVNBX1RBU0tfRVhF"
            "Q19UQVJHRVQiOiAiQUxJIn0&with=eyJFeGVjIjogIndlYy5zaCI"
            "sICJQbHVnaW5OYW1lIjogIndwZSIsICJDdXN0b21lcklkIjogIndjZCJ9")


class TestConfig(unittest.TestCase):
    def test_encode_json_base64(self):
        params = dict()
        params["key1"] = "val1"
        params["key2"] = "val2"
        b64 = Config._encode_json_base64(params)
        self.assertEqual("eyJrZXkyIjogInZhbDIiLCAia2V5MSI6ICJ2YWwxIn0", b64)

        params = Config._decode_json_base64(b64)
        self.assertEqual(2, len(params))
        self.assertEqual("val1", params["key1"])
        self.assertEqual("val2", params["key2"])

    def test_dsn_parsing(self):
        cfg = Config(test_url)
        self.assertEqual("pid", cfg.pop_access_id)
        self.assertEqual("psc", cfg.pop_access_secret)
        self.assertEqual("jtest_env", cfg.curr_project)
        self.assertEqual("http", cfg.scheme)
        self.assertEqual("wcd", cfg.withs["CustomerId"])
        self.assertEqual("wpe", cfg.withs["PluginName"])
        self.assertEqual("wec.sh", cfg.withs["Exec"])
        self.assertEqual("SKY", cfg.env["SKYNET_ACCESSKEY"])

    def test_to_dsn(self):
        cfg = Config(test_url)
        url = cfg.to_url()
        self.assertEqual(test_url, url)

    def test_parse_error(self):
        # no env and with
        dsn = "alisa://pid:psc@dw.a.hk/?scheme=http&verbose=true"
        self.assertRaises(ValueError, lambda: Config(dsn))


if __name__ == "__main__":
    unittest.main()
