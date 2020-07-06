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

import http.client
import socket
import threading
import time
from contextlib import closing
from http.server import BaseHTTPRequestHandler, HTTPServer

from runtime.xgboost.tracker import RabitTracker


class PaiXGBoostWorker():
    @staticmethod
    def read_tracker_port(host, port, ttl=900):
        started = int(time.time())
        each_sleep = 30
        while True:
            if int(time.time()) - started > ttl:
                raise Exception(
                    "failed to read the tracker port in {} seconds".format(
                        ttl))
            conn = None
            try:
                conn = http.client.HTTPConnection(host=host,
                                                  port=port,
                                                  timeout=60)
                conn.request('GET', '/')
                rsp = conn.getresponse()
                return str(rsp.read())
            except Exception:
                time.sleep(each_sleep)
            finally:
                if conn is not None:
                    conn.close()

    @staticmethod
    def gen_envs(host, port, ttl, nworkers, task_id):
        tracker_port = PaiXGBoostWorker.read_tracker_port(host, port, ttl)
        if tracker_port == "":
            raise Exception("bad response from tracker, empty port")
        return [
            str.encode('DMLC_NUM_WORKER=%d' % nworkers),
            str.encode('DMLC_TRACKER_URI=%s' % host),
            str.encode('DMLC_TRACKER_PORT=%s' % tracker_port),
            str.encode('DMLC_TASK_ID=%d' % task_id)
        ]


class PaiXGBoostTracker(object):
    def __init__(self, host, nworkers, port):
        self.httpd = HTTPServer((host, port),
                                self.TrackerHelperHTTPRequestHandler)
        tracker_port = PaiXGBoostTracker.find_free_port()
        nslave = nworkers
        tracker = RabitTracker(host, nslave, tracker_port, tracker_port + 1)
        tracker.start(nslave)
        self.tracker = tracker
        self.TrackerHelperHTTPRequestHandler.tracker_port = str(tracker.port)
        threading.Thread(target=self.httpd.serve_forever).start()

    def join(self):
        self.tracker.join()
        self.httpd.shutdown()
        self.httpd.server_close()

    @staticmethod
    def find_free_port():
        with closing(socket.socket(socket.AF_INET, socket.SOCK_STREAM)) as s:
            s.bind(('', 0))
            s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            return s.getsockname()[1]

    class TrackerHelperHTTPRequestHandler(BaseHTTPRequestHandler):
        tracker_port = ""

        def do_GET(self):
            self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            self.wfile.write(self.tracker_port.encode("utf-8"))
