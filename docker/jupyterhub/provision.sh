#!/bin/sh

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

# This script is to do some Playground initialization, like change logo,
# Add homepage content, etc.

# Change logo to SQLFLow
sed -i -e \
    's@{{base_url}}logo@https://avatars0.githubusercontent.com/u/48231449?s=200\&v=4@g' \
    /usr/local/share/jupyterhub/templates/page.html

# Add welcome words in homepage
# shellcheck disable=SC1004
sed -i -e 's@{% block main %}@& \n\
<div style="width:100%; text-align:center; margin:15vh 0 -15vh 0;"> \n\
  <span style="font-size: 2em; color: #1BA2FF">SQLFlow</span> \n\
  <div style="font-size: 1.2em; color: #454545">Extends SQL to support AI. Extract knowledge from Data.</div> \n\
  <br/> \n\
  <div> \n\
    <a href="https://sql-machine-learning.github.io/sqlflow/">GitHub</a> \n\
    <span>\&emsp;|\&emsp;</span> \n\
    <a href="https://github.com/sql-machine-learning/sqlflow/wiki/Contact-us">Contact Us</a> \n\
  </div> \n\
</div>@g' /usr/local/share/jupyterhub/templates/login.html

# Add GitHub OAuth retry, we encounter connection failures sometimes, so add retry.
cat >/tmp/retry_code<<EOM
        # retry for 3 times
        retry = 3
        for i in range(retry):
            try:
                resp = await http_client.fetch(req)
                break
            except BaseException as e:
                if i == retry - 1:
                    # (TODO:lhw) we may use local cookie if all retries failed
                    raise e
                self.log.info('Connection fail, sleep 2sec and retry %s', str(e))
                import time
                time.sleep(2)

EOM

# Patch GitHub OAuth, add retry
sed -i -e'160r /tmp/retry_code' -e'160d' \
    /usr/local/lib/python3.8/dist-packages/oauthenticator/github.py
