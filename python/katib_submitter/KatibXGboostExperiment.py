# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from pprint import pprint

from kubernetes import client, config
from kubernetes.client.rest import ApiException

import simplejson as json
import yaml 
import sys
import os

from experiment_spec_helper import *
import katib_client


class KatibXGBoostExperiment:
    
    def __init__(
            self,
            select,
            validate_select,
            feature_names,
            exp_name,
            exp_params
        ):

        self.base_exp_name = 'katib-xgboost' 

        self.select = select
        self.validate_select = validate_select
        self.feature_names = feature_names
        
        self.exp_name = exp_name

        self.booster = 'gtree'
        self.objective = 'binary:logistic'
        self.search_algorithm = 'random'

        print(exp_params)

        for param_name in exp_params:
            if param_name == 'booster':
                self.booster = exp_params[param_name]
            elif param_name == 'objective':
                self.objective = exp_params[param_name]
            elif param_name == 'search_algorithm':
                self.search_algorithm = exp_params[param_name]

        yaml_file = os.path.join(os.path.dirname(__file__), 'template-katib-xgboost.yaml')
        with open(yaml_file) as f:
            experiment_spec = yaml.safe_load(f)

        self.exp_spec = experiment_spec
        self.xgboost_hpo_params = {}
                
    
    def set_hpo_parameter(self, max_depth=[], num_round=[]):
        """
        hyper parameter range for katib to search
        max_depth: [4, 10]
        num_round: [500, 1000]
        
        return:
        """
        self.xgboost_hpo_params['max_depth'] = max_depth
        self.xgboost_hpo_params['num_round'] = num_round

    def submit_xgboost_experiment(self):
        """
        Generate xgboost experiment specific
        """

        self.set_hpo_parameter(max_depth=[1, 12], num_round=[40, 120])
        self._prepare_for_hpo()


        # ff = open('meta.yaml', 'w+')
        # yaml.dump(self.exp_spec, ff, allow_unicode=True)

        
        client = katib_client.KatibClient()
        api_instance = client.get_custom_object_api_client()

        group = 'kubeflow.org' # str | The custom resource's group name
        version = 'v1alpha3' # str | The custom resource's version
        namespace = 'kubeflow' # str | The custom resource's namespace
        plural = 'experiments' # str | The custom resource's plural name. For TPRs this would be lowercase plural kind.
        
        try: 
            api_response = api_instance.create_namespaced_custom_object(group, version, namespace, plural, self.exp_spec)
            pprint(api_response)
        except ApiException as e:
            print("Exception when calling create_namespaced_custom_object: %s\n" % e)
  

    def _prepare_for_hpo(self):
        full_exp_name = self.base_exp_name + "-" + self.exp_name
        set_exp_name(self.exp_spec, full_exp_name)

        self._config_xgboost_experiment()

    def _config_xgboost_experiment(self):
        set_search_algorithm(self.exp_spec, self.search_algorithm)
        set_xgb_hpo_params(self.exp_spec, self.xgboost_hpo_params['max_depth'], self.xgboost_hpo_params['num_round'])
        set_xgb_hpo_args(self.exp_spec, select=self.select, validate_select=self.validate_select, feature_names=self.feature_names, booster=self.booster, objective=self.objective)

    
      