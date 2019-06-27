# Copyright 2019 The SQLFlow Authors. All rights reserved.
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

# Configuration file for jupyterhub.
# shutdown the server after no activity for an hour
import os
c.NotebookApp.shutdown_no_activity_timeout = 60 * 60
c.LocalProcessSpawner.environment = {
    "SQLFLOW_DATASOURCE": "mysql://root:root@tcp(%s:%s)/?maxAllowedPacket=0"%(os.getenv("SQLFLOW_MYSQL_SERVICE_HOST", ""), os.getenv("SQLFLOW_MYSQL_SERVICE_PORT", "3306")),
    "SQLFLOW_SERVER": "%s:%s" % (os.getenv("SQLFLOW_SERVER_SERVICE_HOST", ""), os.getenv("SQLFLOW_SERVER_SERVICE_PORT", ""))
}
c.LocalProcessSpawner.cmd = ["/miniconda/envs/sqlflow-dev/bin/jupyterhub-singleuser"]