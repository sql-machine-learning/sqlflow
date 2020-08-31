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
'''This Module implements SQLFlow Step in Couler'''

from os import path

import couler.argo as couler


def escape_sql(original_sql):
    '''Escape special chars in SQL'''
    return original_sql.replace('\\', '\\\\').replace('"', r'\"').replace(
        "`", r'\`').replace("$", r'\$')


def sqlflow(sql,
            image="sqlflow/sqlflow",
            env=None,
            secret=None,
            resources=None,
            log_file=None):
    '''sqlflow step call run_container to append a workflow step.
    '''
    if not log_file:
        command = '''step -e "%s"''' % escape_sql(sql)
    else:
        log_dir = path.dirname(log_file)
        command = '''(mkdir -p %s && step -e "%s") 2>&1 | tee %s''' % (
            log_dir, escape_sql(sql), log_file)

    couler.run_container(command=command,
                         image=image,
                         env=env,
                         secret=secret,
                         resources=resources)
