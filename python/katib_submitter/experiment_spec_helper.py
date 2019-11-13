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

import yaml

def set_exp_name(spec, exp_name):
    spec['metadata']['name'] = exp_name

def set_search_algorithm(spec, algorithm):
    spec['spec']['algorithm']['algorithmName'] = algorithm

def set_xgb_hpo_params(spec, max_depth, num_round):

    parameters = spec['spec']['parameters']

    for para in parameters:
        if para['name'] == "--max_depth":
            config = para['feasibleSpace']
            config['min'] = '"' + str(max_depth[0]) + '"'
            config['max'] = '"' + str(max_depth[1]) + '"'

        elif para['name'] == "--num_round":
            config = para['feasibleSpace']
            config['min'] = '"' + str(num_round[0]) + '"'
            config['max'] = '"' + str(num_round[1]) + '"'

def set_xgb_hpo_args(
        exp_spec, 
        select='', 
        validate_select='', 
        feature_names='', 
        booster='gtree', 
        objective='binary:logistic'
    ):
    pod_template = exp_spec['spec']['trialTemplate']['goTemplate']['rawTemplate']
    #print(pod_template)

    select_param = "--select %s" % select.replace(' ', '+')
    pod_template = pod_template.replace("--select", select_param)

    validate_select_param = '--validate_select %s' % validate_select.replace(' ', '+')
    pod_template = pod_template.replace('--validate_select', validate_select_param)

    feature_name_param = '--feature_name %s' % feature_names
    pod_template = pod_template.replace('--feature_names', feature_name_param)
    if booster == 'gtree':
        booster_param = '--booster %s' % booster
        pod_template = pod_template.replace('--booster', booster_param)

    objective_param = '--objective %s' % objective
    pod_template = pod_template.replace('--objective', objective_param)

    #print(pod_template)

    exp_spec['spec']['trialTemplate']['goTemplate']['rawTemplate'] = pod_template 



     