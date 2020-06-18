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

import pandas as pd
import pyomo.environ as pyomo_env
from sqlflow_submitter.optimize.optimize import (
    generate_model_with_data_frame, generate_objective_or_constraint_func,
    generate_range_constraint_func)


def get_source(func):
    code = func.code
    code = code[code.index(":") + 1:]
    return code.strip()


class TestModelGenerationBase(unittest.TestCase):
    def generate_objective_func(self, objective, result_value_name):
        return generate_objective_or_constraint_func(
            expression=objective,
            data_frame=self.data_frame,
            variables=self.variables,
            result_value_name=result_value_name)

    def generate_constraint_func(self,
                                 constraint,
                                 result_value_name,
                                 is_aggregation=True,
                                 index=None):
        if is_aggregation:
            return generate_objective_or_constraint_func(
                expression=constraint["expression"],
                data_frame=self.data_frame,
                variables=self.variables,
                result_value_name=result_value_name,
                index=index)
        else:
            return generate_range_constraint_func(
                expression=constraint["expression"],
                data_frame=self.data_frame,
                variables=self.variables,
                result_value_name=result_value_name)


class TestModelGenerationWithoutGroupBy(TestModelGenerationBase):
    def setUp(self):
        self.data_frame = pd.DataFrame(
            data={
                'product': ['soldier', 'train'],
                'price': [27, 21],
                'materials_cost': [10, 9],
                'other_cost': [14, 10],
                'finishing': [2, 1],
                'carpentry': [1, 1],
                'max_num': [40, 10000],
            })

        self.variables = ["product"]

    def test_multiple_brackets(self):
        constraint = {
            "expression": [
                'SUM', '(', 'finishing', '*', 'product', '+', 'SUM', '(',
                'product', ')', ')', '<=', '100'
            ]
        }
        c0 = self.generate_constraint_func(constraint,
                                           result_value_name='product')
        c1 = self.generate_constraint_func(constraint,
                                           result_value_name="product_value")
        self.assertEqual(get_source(c0), get_source(c1))
        self.assertEqual(
            get_source(c0),
            "sum([DATA_FRAME.finishing[i_0]*model.x[i_0]+sum([model.x[i_1] for i_1 in model.x]) for i_0 in model.x])<=100"
        )

    def test_model_generation(self):
        objective = [
            'SUM', '(', '(', 'price', '-', 'materials_cost', '-', 'other_cost',
            ')', '*', 'product', ')'
        ]
        constraints = [
            {
                "expression":
                ['SUM', '(', 'finishing', '*', 'product', ')', '<=', '100'],
                "group_by":
                "",
            },
            {
                "expression":
                ['SUM', '(', 'carpentry', '*', 'product', ')', '<=', '80'],
                "group_by":
                ""
            },
            {
                "expression": ['product', '<=', 'max_num']
            },
        ]

        obj_func1 = self.generate_objective_func(objective, "product_value")
        obj_func2 = self.generate_objective_func(objective, "product")
        self.assertEqual(get_source(obj_func1), get_source(obj_func2))
        self.assertEqual(
            get_source(obj_func1),
            "sum([(DATA_FRAME.price[i_0]-DATA_FRAME.materials_cost[i_0]-DATA_FRAME.other_cost[i_0])*model.x[i_0] for i_0 in model.x])"
        )

        const_01 = self.generate_constraint_func(constraints[0],
                                                 "product_value")
        const_02 = self.generate_constraint_func(constraints[0], "product")
        self.assertEqual(get_source(const_01), get_source(const_02))
        self.assertEqual(
            get_source(const_01),
            "sum([DATA_FRAME.finishing[i_0]*model.x[i_0] for i_0 in model.x])<=100"
        )

        const_11 = self.generate_constraint_func(constraints[1],
                                                 "product_value")
        const_12 = self.generate_constraint_func(constraints[1], "product")
        self.assertEqual(get_source(const_11), get_source(const_12))
        self.assertEqual(
            get_source(const_11),
            "sum([DATA_FRAME.carpentry[i_0]*model.x[i_0] for i_0 in model.x])<=80"
        )

        const_21 = self.generate_constraint_func(constraints[1],
                                                 "product_value", False)
        const_22 = self.generate_constraint_func(constraints[1], "product",
                                                 False)
        self.assertEqual(get_source(const_21), get_source(const_22))
        self.assertEqual(get_source(const_21),
                         "SUM(DATA_FRAME.carpentry[i]*model.x[i])<=80")

        # TODO(sneaxiy): need to add more tests to generated models
        model1 = generate_model_with_data_frame(data_frame=self.data_frame,
                                                variables=self.variables,
                                                variable_type="Integers",
                                                result_value_name="product",
                                                objective=objective,
                                                direction="maximize",
                                                constraints=constraints)
        self.assertTrue(isinstance(model1, pyomo_env.ConcreteModel))

        model2 = generate_model_with_data_frame(
            data_frame=self.data_frame,
            variables=self.variables,
            variable_type="Reals",
            result_value_name="product_value",
            objective=objective,
            direction="minimize",
            constraints=constraints)
        self.assertTrue(isinstance(model2, pyomo_env.ConcreteModel))


class TestModelGenerationWithGroupBy(TestModelGenerationBase):
    def setUp(self):
        self.data_frame = pd.DataFrame(
            data={
                'plants': ["plantA", "plantA", "plantB", "plantB"],
                'markets': ["marketA", "marketB", "marketA", "marketB"],
                'distance': [140, 210, 300, 90],
                'capacity': [100, 100, 90, 90],
                'demand': [130, 60, 130, 60]
            })

        self.variables = ["plants", "markets"]
        self.result_value_name = "shipment"

    def test_main(self):
        objective = [
            'SUM', '(', 'distance', '*', 'shipment', '*', '90', '/', '1000',
            ')'
        ]

        constraints = [
            {
                "expression": ['SUM', '(', 'shipment', ')', '<=', 'capacity'],
                "group_by": "plants",
            },
            {
                "expression": ['SUM', '(', 'shipment', ')', '>=', 'demand'],
                "group_by": "markets",
            },
            {
                "expression": ['shipment', '*', '2', '<=', 'demand'],
                "group_by": "markets",
            },
        ]

        obj_func = self.generate_objective_func(objective,
                                                self.result_value_name)
        self.assertEqual(
            get_source(obj_func),
            "sum([DATA_FRAME.distance[i_0]*model.x[i_0]*90/1000 for i_0 in model.x])"
        )

        const_0 = self.generate_constraint_func(constraints[0],
                                                self.result_value_name,
                                                index=[0, 1])
        self.assertEqual(get_source(const_0),
                         "sum([model.x[i_0] for i_0 in [0, 1]])<=100")

        const_1 = self.generate_constraint_func(constraints[1],
                                                self.result_value_name,
                                                index=[2, 3])
        self.assertEqual(get_source(const_1),
                         "sum([model.x[i_0] for i_0 in [2, 3]])>=130")

        const_2 = self.generate_constraint_func(constraints[2],
                                                self.result_value_name,
                                                index=[2, 3],
                                                is_aggregation=False)
        self.assertEqual(get_source(const_2),
                         "model.x[i]*2<=DATA_FRAME.demand[i]")

        model = generate_model_with_data_frame(
            data_frame=self.data_frame,
            variables=self.variables,
            variable_type="NonNegativeIntegers",
            result_value_name=self.result_value_name,
            objective=objective,
            direction="minimize",
            constraints=constraints)
        self.assertTrue(isinstance(model, pyomo_env.ConcreteModel))


if __name__ == '__main__':
    unittest.main()
