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

import unittest

from runtime.optimize.optflow import \
    generate_optflow_fsl_expr_when_two_vars as gen_expr


class TestOptFlowFSLGeneration(unittest.TestCase):
    def test_shipment_case(self):
        columns = ['plants', 'markets', 'distance', 'capacity', 'demand']
        variables = ['plants', 'markets']
        result_value_name = 'shipment'

        obj_tokens = [
            'SUM', '(', 'shipment', '*', 'distance', '*', '90', '/', '1000',
            ')'
        ]

        obj_expr = gen_expr(columns=columns,
                            tokens=obj_tokens,
                            variables=variables,
                            result_value_name=result_value_name)
        self.assertEqual(
            obj_expr, 'sum([@X[i,j]*@input["distance"][i,j]*90/1000 '
            'for i in @I for j in @J])')

        constraint_tokens = ['SUM', '(', 'shipment', ')', '<=', 'capacity']
        c_expr = gen_expr(columns=columns,
                          tokens=constraint_tokens,
                          variables=variables,
                          result_value_name=result_value_name,
                          group_by='plants')
        self.assertEqual(
            c_expr, 'for i in @I: sum([@X[i,j] for j in @J])'
            '<=@input["capacity"][i,@J[0]]')

        constraint_tokens = ['SUM', '(', 'shipment', ')', '>=', 'demand']
        c_expr = gen_expr(columns=columns,
                          tokens=constraint_tokens,
                          variables=variables,
                          result_value_name=result_value_name,
                          group_by='markets')
        self.assertEqual(
            c_expr, 'for j in @J: sum([@X[i,j] for i in @I])'
            '>=@input["demand"][@I[0],j]')

    def test_asset_case(self):
        columns = [
            'user_id', 'org_id', 'max_num', 'k', 'c', 'u', 'p',
            'asset_at_risk', 'asset'
        ]
        variables = ['user_id', 'org_id']
        result_value_name = 'x'

        obj_tokens = ['SUM', '(', 'k', '*', 'x', ')']

        obj_expr = gen_expr(columns=columns,
                            tokens=obj_tokens,
                            variables=variables,
                            result_value_name=result_value_name)
        self.assertEqual(
            obj_expr, 'sum([@input["k"][i,j]*@X[i,j] '
            'for i in @I for j in @J])')

        constraint_tokens = ['SUM', '(', 'x', ')', '<=', 'max_num']
        c_expr = gen_expr(columns=columns,
                          tokens=constraint_tokens,
                          variables=variables,
                          result_value_name=result_value_name,
                          group_by='org_id')
        self.assertEqual(
            c_expr, 'for j in @J: sum([@X[i,j] for i in @I])'
            '<=@input["max_num"][@I[0],j]')

        constraint_tokens = ['SUM', '(', 'x', ')', '=', '1']
        c_expr = gen_expr(columns=columns,
                          tokens=constraint_tokens,
                          variables=variables,
                          result_value_name=result_value_name,
                          group_by='user_id')
        self.assertEqual(c_expr, 'for i in @I: sum([@X[i,j] for j in @J])==1')

        constraint_tokens = [
            'SUM', '(', 'c', '*', 'u', '*', 'p', '*', 'x', ')', '+',
            'asset_at_risk', '<=', '0.055', '*', '(', 'SUM', '(', 'c', '*',
            'u', '*', 'x', ')', '+', 'asset', ')'
        ]
        c_expr = gen_expr(columns=columns,
                          tokens=constraint_tokens,
                          variables=variables,
                          result_value_name=result_value_name,
                          group_by='user_id')
        self.assertEqual(
            c_expr, 'for i in @I: sum([@input["c"][i,j]*@input["u"][i,j]*'
            '@input["p"][i,j]*@X[i,j] for j in @J])+'
            '@input["asset_at_risk"][i,@J[0]]'
            '<=0.055*(sum([@input["c"][i,j]*@input["u"][i,j]*@X[i,j] '
            'for j in @J])+@input["asset"][i,@J[0]])')


if __name__ == '__main__':
    unittest.main()
