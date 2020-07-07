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

import numpy as np
import pandas as pd
import pyomo.environ as pyomo_env
from sqlflow_submitter.optimize.optimize import (
    generate_model_with_data_frame,
    generate_objective_or_constraint_expressions, solve_model)


class TestModelGenerationBase(unittest.TestCase):
    def generate_objective(self,
                           tokens,
                           result_value_name,
                           variable_str="model.x",
                           data_frame_str="DATA_FRAME"):
        expressions = generate_objective_or_constraint_expressions(
            tokens=tokens,
            data_frame=self.data_frame,
            variables=self.variables,
            result_value_name=result_value_name,
            variable_str=variable_str,
            data_frame_str=data_frame_str)
        self.assertEqual(len(expressions), 1)
        self.assertEqual(len(expressions[0]), 1)
        return expressions[0][0]

    def generate_constraints(self, constraint, result_value_name):
        expressions = generate_objective_or_constraint_expressions(
            tokens=constraint.get("tokens"),
            data_frame=self.data_frame,
            variables=self.variables,
            result_value_name=result_value_name,
            group_by=constraint.get("group_by"))
        return expressions


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
            "tokens": [
                'SUM', '(', 'finishing', '*', 'product', '+', 'SUM', '(',
                'product', ')', ')', '<=', '100'
            ]
        }
        c0 = self.generate_constraints(constraint, result_value_name='product')
        c1 = self.generate_constraints(constraint,
                                       result_value_name="product_value")
        self.assertEqual(len(c0), 1)
        self.assertEqual(len(c0[0]), 1)
        self.assertEqual(len(c1), 1)
        self.assertEqual(len(c1[0]), 1)
        self.assertEqual(c0[0][0], c1[0][0])

        self.assertEqual(
            c0[0][0],
            'sum([DATA_FRAME["finishing"][i_0]*model.x[i_0]+sum([model.x[i_1] for i_1 in model.x]) for i_0 in model.x])<=100'
        )

    def test_model_generation(self):
        objective = [
            'SUM', '(', '(', 'price', '-', 'materials_cost', '-', 'other_cost',
            ')', '*', 'product', ')'
        ]
        constraints = [
            {
                "tokens":
                ['SUM', '(', 'finishing', '*', 'product', ')', '<=', '100'],
            },
            {
                "tokens":
                ['SUM', '(', 'carpentry', '*', 'product', ')', '<=', '80'],
            },
            {
                "tokens": ['product', '<=', 'max_num']
            },
        ]

        obj_str1 = self.generate_objective(objective, "product_value")
        obj_str2 = self.generate_objective(objective, "product")
        self.assertEqual(obj_str1, obj_str2)
        self.assertEqual(
            obj_str1,
            'sum([(DATA_FRAME["price"][i_0]-DATA_FRAME["materials_cost"][i_0]-DATA_FRAME["other_cost"][i_0])*model.x[i_0] for i_0 in model.x])'
        )

        const_01 = self.generate_constraints(constraints[0], "product_value")
        const_02 = self.generate_constraints(constraints[0], "product")
        self.assertEqual(len(const_01), 1)
        self.assertEqual(len(const_01[0]), 1)
        self.assertEqual(len(const_02), 1)
        self.assertEqual(len(const_02[0]), 1)
        self.assertEqual(const_01[0][0], const_02[0][0])
        self.assertEqual(
            const_01[0][0],
            'sum([DATA_FRAME["finishing"][i_0]*model.x[i_0] for i_0 in model.x])<=100'
        )

        const_11 = self.generate_constraints(constraints[1], "product_value")
        const_12 = self.generate_constraints(constraints[1], "product")
        self.assertEqual(len(const_11), 1)
        self.assertEqual(len(const_11[0]), 1)
        self.assertEqual(len(const_12), 1)
        self.assertEqual(len(const_12[0]), 1)
        self.assertEqual(const_11[0][0], const_12[0][0])
        self.assertEqual(
            const_11[0][0],
            'sum([DATA_FRAME["carpentry"][i_0]*model.x[i_0] for i_0 in model.x])<=80'
        )

        const_21 = self.generate_constraints(constraints[2], "product_value")
        const_22 = self.generate_constraints(constraints[2], "product")
        self.assertEqual(len(const_21), 1)
        self.assertEqual(len(const_21[0]), 2)
        self.assertTrue(const_21[0][1] is None)
        self.assertEqual(len(const_22), 1)
        self.assertEqual(len(const_22[0]), 2)
        self.assertTrue(const_22[0][1] is None)
        self.assertEqual(const_21[0][0], const_22[0][0])
        self.assertEqual(const_21[0][0],
                         'model.x[i]<=DATA_FRAME["max_num"][i]')

        # TODO(sneaxiy): need to add more tests to generated models
        model1 = generate_model_with_data_frame(data_frame=self.data_frame,
                                                variables=self.variables,
                                                variable_type="Integers",
                                                result_value_name="product",
                                                objective=objective,
                                                direction="maximize",
                                                constraints=constraints)
        self.assertTrue(isinstance(model1, pyomo_env.ConcreteModel))
        result = solve_model(model1, 'glpk')
        self.assertTrue(
            np.array_equal(result, np.array([20, 60], dtype='int64')))

        model2 = generate_model_with_data_frame(
            data_frame=self.data_frame,
            variables=self.variables,
            variable_type="Reals",
            result_value_name="product_value",
            objective=objective,
            direction="minimize",
            constraints=constraints)
        self.assertTrue(isinstance(model2, pyomo_env.ConcreteModel))

        with self.assertRaises(ValueError):
            solve_model(model2, 'glpk')


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
                "tokens": ['SUM', '(', 'shipment', ')', '<=', 'capacity'],
                "group_by": "plants",
            },
            {
                "tokens": ['SUM', '(', 'shipment', ')', '>=', 'demand'],
                "group_by": "markets",
            },
            {
                "tokens": ['shipment', '*', '100', '>=', 'demand'],
            },
        ]

        obj_func = self.generate_objective(objective, self.result_value_name)
        self.assertEqual(
            obj_func,
            'sum([DATA_FRAME["distance"][i_0]*model.x[i_0]*90/1000 for i_0 in model.x])'
        )

        const_0 = self.generate_constraints(constraints[0],
                                            self.result_value_name)
        self.assertEqual(len(const_0), 2)
        self.assertEqual(len(const_0[0]), 1)
        self.assertEqual(len(const_0[1]), 1)

        self.assertEqual(const_0[0][0],
                         "sum([model.x[i_0] for i_0 in [0, 1]])<=100")
        self.assertEqual(const_0[1][0],
                         "sum([model.x[i_0] for i_0 in [2, 3]])<=90")

        const_1 = self.generate_constraints(constraints[1],
                                            self.result_value_name)
        self.assertEqual(len(const_1), 2)
        self.assertEqual(len(const_1[0]), 1)
        self.assertEqual(len(const_1[1]), 1)
        self.assertEqual(const_1[0][0],
                         "sum([model.x[i_0] for i_0 in [0, 2]])>=130")
        self.assertEqual(const_1[1][0],
                         "sum([model.x[i_0] for i_0 in [1, 3]])>=60")

        const_2 = self.generate_constraints(constraints[2],
                                            self.result_value_name)
        self.assertEqual(len(const_2), 1)
        self.assertEqual(len(const_2[0]), 2)
        self.assertEqual(const_2[0][0],
                         'model.x[i]*100>=DATA_FRAME["demand"][i]')
        self.assertTrue(const_2[0][1] is None)

        model = generate_model_with_data_frame(
            data_frame=self.data_frame,
            variables=self.variables,
            variable_type="NonNegativeIntegers",
            result_value_name=self.result_value_name,
            objective=objective,
            direction="minimize",
            constraints=constraints)
        self.assertTrue(isinstance(model, pyomo_env.ConcreteModel))

        result = solve_model(model, 'glpk')
        self.assertTrue(
            np.array_equal(result, np.array([99, 1, 31, 59], dtype='int64')))


if __name__ == '__main__':
    unittest.main()
