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

# pylint: disable-all
# flake8: noqa

# Configuration file for jupyterhub.
import os
import socket
import sys

from dummyauthenticator import DummyAuthenticator
from kubernetes import client
from oauthenticator.github import GitHubOAuthenticator

sqlflow_jupyter_image = os.getenv("SQLFLOW_JUPYTER_IMAGE")
sqlflow_mysql_image = os.getenv("SQLFLOW_MYSQL_IMAGE")

c.JupyterHub.spawner_class = 'kubespawner.KubeSpawner'

c.JupyterHub.ip = '0.0.0.0'
c.JupyterHub.hub_ip = '0.0.0.0'

ssl_key_path = os.getenv("SQLFLOW_JUPYTER_SSL_KEY")
ssl_cert_path = os.getenv("SQLFLOW_JUPYTER_SSL_CERT")
if ssl_key_path != "" and ssl_cert_path != "":
    c.JupyterHub.ssl_key = ssl_key_path
    c.JupyterHub.ssl_cert = ssl_cert_path

# Don't try to cleanup servers on exit - since in general for k8s, we want
# the hub to be able to restart without losing user containers
c.JupyterHub.cleanup_servers = False

# First pulls can be really slow, so let's give it a big timeout
c.KubeSpawner.start_timeout = 60 * 10

# Find the IP of the machine that minikube is most likely able to talk to
# Graciously used from https://stackoverflow.com/a/166589
s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.connect(("8.8.8.8", 80))
host_ip = s.getsockname()[0]
s.close()

c.KubeSpawner.hub_connect_ip = host_ip
c.JupyterHub.hub_connect_ip = c.KubeSpawner.hub_connect_ip

c.KubeSpawner.service_account = 'default'

# use GitHub oauth
if os.getenv("SQLFLOW_JUPYTER_USE_GITHUB_OAUTH") == "true":
    c.JupyterHub.authenticator_class = GitHubOAuthenticator
    c.GitHubOAuthenticator.oauth_callback_url = 'https://playground.sqlflow.tech/hub/oauth_callback'
    c.GitHubOAuthenticator.client_id = os.getenv(
        "SQLFLOW_JUPYTER_OAUTH_CLIENT_ID")
    c.GitHubOAuthenticator.client_secret = os.getenv(
        "SQLFLOW_JUPYTER_OAUTH_CLIENT_SECRET")
else:
    c.JupyterHub.authenticator_class = DummyAuthenticator
    # c.Authenticator.admin_users = {'put your admin user here'}

c.JupyterHub.allow_named_servers = True

# To speed up container booting, we use local cache.
# Addintonally, we add a cron task to pull image daily.
c.KubeSpawner.image_pull_policy = 'IfNotPresent'
c.KubeSpawner.storage_pvc_ensure = False
c.KubeSpawner.extra_pod_config.update({'restartPolicy': 'Never'})

# container tonyyang/sqlflow:sqlflow need to be run at root to start MySQL
c.KubeSpawner.uid = 0
c.KubeSpawner.profile_list = [{
    'display_name':
    'SQLFlow Playground',
    'default':
    True,
    'kubespawner_override': {
        'image': sqlflow_jupyter_image,
    },
    'description':
    'Brings SQL and AI together. <a href="https://sqlflow.org">https://sqlflow.org</a>'
}]
c.KubeSpawner.cmd = [
    "sh", "-c",
    "export SQLFLOW_DATASOURCE=mysql://root:root@tcp\(${MY_POD_IP}:3306\)/?maxAllowedPacket=0 && \
    export SQLFLOW_SERVER=${SQLFLOW_SERVER_SERVICE_HOST}:${SQLFLOW_SERVER_SERVICE_PORT} && jupyterhub-singleuser jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --notebook-dir=/workspace/"
]

c.KubeSpawner.extra_containers = [{
    "name":
    "sqlflow",
    "image":
    sqlflow_mysql_image,
    "imagePullPolicy":
    "IfNotPresent",
    "livenessProbe": {
        "exec": {
            "command": ["cat", "/work/mysql-inited"]
        },
        "initialDelaySeconds": 600,
        "periodSeconds": 60,
    },
    "env": [{
        "name": "MYSQL_HOST",
        "value": "0.0.0.0"
    }, {
        "name": "MYSQL_PORT",
        "value": "3306"
    }],
    "ports": [{
        "containerPort": 3306,
    }]
}]

# shutdown the server after no activity for an hour
c.JupyterHub.services = [{
    'name':
    'idle-culler',
    'admin':
    True,
    'command':
    [sys.executable, '-m', 'jupyterhub_idle_culler', '--timeout=3600'],
}]


def modify_pod_hook(spawner, pod):
    pod.spec.containers[0].env.append(
        client.V1EnvVar(
            "MY_POD_IP", None,
            client.V1EnvVarSource(
                None, client.V1ObjectFieldSelector(None, "status.podIP"))))
    return pod


c.KubeSpawner.modify_pod_hook = modify_pod_hook
