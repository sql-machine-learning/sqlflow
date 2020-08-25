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

import copy
import unittest

import numpy as np
import pandas as pd
import pyomo.environ as pyomo_env
from runtime.optimize.local import generate_model_with_data_frame, solve_model
from runtime.optimize.model_generation import (
    IDENTIFIER_REGEX, assert_are_valid_tokens,
    generate_objective_and_constraint_expr)


class TestAssertValidTokens(unittest.TestCase):
    def is_identifier(self, token):
        return IDENTIFIER_REGEX.fullmatch(token) is not None

    def test_is_identifier(self):
        tokens = ['a', '_', 'a123', '__', '_123']
        for t in tokens:
            self.assertTrue(self.is_identifier(t))

        tokens = ['1', '123_', '3def']
        for t in tokens:
            self.assertFalse(self.is_identifier(t))

    def test_assert_valid_tokens(self):
        tokens = ['SUM', '(', 'finishing', '*', 'product', ')', '<=', '100']

        # valid expression
        assert_are_valid_tokens(columns=['finishing', 'product'],
                                tokens=tokens,
                                result_value_name='product')

        # invalid group_by
        with self.assertRaises(AssertionError):
            assert_are_valid_tokens(columns=['finishing', 'product'],
                                    tokens=tokens,
                                    result_value_name='product',
                                    group_by='invalid_group_by')

        # tokens = None
        with self.assertRaises(AssertionError):
            assert_are_valid_tokens(columns=['finishing', 'product'],
                                    tokens=None,
                                    result_value_name='product')

        # tokens = []
        with self.assertRaises(AssertionError):
            assert_are_valid_tokens(columns=['finishing', 'product'],
                                    tokens=[],
                                    result_value_name='product')

        # tokens not inside columns
        tokens = [
            'SUM', '(', 'finishing', '*', 'invalid_token', ')', '<=', '100'
        ]
        with self.assertRaises(AssertionError):
            assert_are_valid_tokens(columns=['finishing', 'product'],
                                    tokens=tokens,
                                    result_value_name='product')

        # tokens not inside columns but equal to result_value_name
        # ignore cases
        tokens = [
            'SUM', '(', 'FinisHing', '*', 'pRoducT_VaLue', ')', '<=', '100'
        ]
        assert_are_valid_tokens(columns=['finishing', 'product'],
                                tokens=tokens,
                                result_value_name='product_value')


class TestModelGenerationBase(unittest.TestCase):
    def generate_objective(self, tokens, result_value_name):
        obj_expr, _ = generate_objective_and_constraint_expr(
            columns=self.data_frame.columns,
            objective=tokens,
            constraints=None,
            variables=self.variables,
            result_value_name=result_value_name,
            variable_str="model.x",
            data_str="DATA_FRAME")
        return obj_expr

    def generate_constraints(self, constraint, result_value_name):
        _, c_expr = generate_objective_and_constraint_expr(
            columns=self.data_frame.columns,
            objective=None,
            constraints=[constraint],
            variables=self.variables,
            result_value_name=result_value_name,
            variable_str="model.x",
            data_str="DATA_FRAME")
        assert len(c_expr) == 1, "invalid constraint expression"
        return c_expr[0]


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

    def replace_objective_token(self, objective, old, new):
        o = copy.copy(objective)
        for i, token in enumerate(o):
            if token == old:
                o[i] = new

        return o

    def replace_constraint_token(self, constraint, old, new):
        def replace_one_constraint(c):
            c = copy.deepcopy(c)
            for i, token in enumerate(c["tokens"]):
                if token == old:
                    c["tokens"][i] = new

            return c

        if isinstance(constraint, (list, tuple)):
            return [replace_one_constraint(c) for c in constraint]
        else:
            return replace_one_constraint(constraint)

    def test_multiple_brackets(self):
        constraint = {
            "tokens": [
                'SUM', '(', 'finishing', '*', 'product', '+', 'SUM', '(',
                'product', ')', ')', '<=', '100'
            ]
        }
        c0, range0, vars0 = self.generate_constraints(
            constraint, result_value_name='product')

        result_value_name = "product_value"
        c1, range1, vars1 = self.generate_constraints(
            self.replace_constraint_token(constraint, "product",
                                          result_value_name),
            result_value_name)

        self.assertEqual(c0, c1)
        self.assertEqual(range0, range1)
        self.assertEqual(vars0, vars1)
        self.assertTrue(vars0 is None)
        self.assertTrue(range0 is None)

        self.assertEqual(
            c0,
            'sum([DATA_FRAME["finishing"][i_0]*model.x[i_0]+sum([model.x[i_1] '
            'for i_1 in model.x]) for i_0 in model.x])<=100')

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

        result_value_name = "product_value"
        obj_str1 = self.generate_objective(
            self.replace_objective_token(objective, "product",
                                         result_value_name), result_value_name)
        obj_str2 = self.generate_objective(objective, "product")
        self.assertEqual(obj_str1, obj_str2)
        self.assertEqual(
            obj_str1,
            'sum([(DATA_FRAME["price"][i_0]-DATA_FRAME["materials_cost"][i_0]-'
            'DATA_FRAME["other_cost"][i_0])*model.x[i_0] for i_0 in model.x])')

        const_01, range_01, vars_01 = self.generate_constraints(
            self.replace_constraint_token(constraints[0], "product",
                                          result_value_name),
            result_value_name)
        const_02, range_02, vars_02 = self.generate_constraints(
            constraints[0], "product")
        self.assertEqual(const_01, const_02)
        self.assertEqual(range_01, range_02)
        self.assertEqual(vars_01, vars_02)
        self.assertTrue(range_01 is None)
        self.assertTrue(vars_01 is None)

        self.assertEqual(
            const_01, 'sum([DATA_FRAME["finishing"][i_0]*model.x[i_0] '
            'for i_0 in model.x])<=100')

        const_11, range_11, vars_11 = self.generate_constraints(
            self.replace_constraint_token(constraints[1], "product",
                                          result_value_name),
            result_value_name)
        const_12, range_12, vars_12 = self.generate_constraints(
            constraints[1], "product")
        self.assertEqual(const_11, const_12)
        self.assertEqual(range_11, range_12)
        self.assertEqual(vars_11, vars_12)
        self.assertTrue(range_11 is None)
        self.assertTrue(vars_11 is None)

        self.assertEqual(
            const_11, 'sum([DATA_FRAME["carpentry"][i_0]*model.x[i_0] '
            'for i_0 in model.x])<=80')

        const_21, range_21, vars_21 = self.generate_constraints(
            self.replace_constraint_token(constraints[2], "product",
                                          result_value_name),
            result_value_name)
        const_22, range_22, vars_22 = self.generate_constraints(
            constraints[2], "product")
        self.assertEqual(const_21, const_22)
        self.assertEqual(range_21, range_22)
        self.assertEqual(vars_21, vars_22)
        self.assertEqual(range_21, "model.x")
        self.assertEqual(vars_21, ["__index"])
        self.assertEqual(const_21,
                         'model.x[__index]<=DATA_FRAME["max_num"][__index]')

        # TODO(sneaxiy): need to add more tests to generated models
        model1 = generate_model_with_data_frame(data_frame=self.data_frame,
                                                variables=self.variables,
                                                variable_type="Integers",
                                                result_value_name="product",
                                                objective=objective,
                                                direction="maximize",
                                                constraints=constraints)
        self.assertTrue(isinstance(model1, pyomo_env.ConcreteModel))
        result_x, result_y = solve_model(model1, 'glpk')
        self.assertTrue(
            np.array_equal(result_x, np.array([20, 60], dtype='int64')))
        self.assertEqual(result_y, 180)

        model2 = generate_model_with_data_frame(
            data_frame=self.data_frame,
            variables=self.variables,
            variable_type="Reals",
            result_value_name=result_value_name,
            objective=self.replace_objective_token(objective, "product",
                                                   result_value_name),
            direction="minimize",
            constraints=self.replace_constraint_token(constraints, "product",
                                                      result_value_name))
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
            obj_func, 'sum([DATA_FRAME["distance"][i_0]*model.x[i_0]*90/1000 '
            'for i_0 in model.x])')

        const_0, range_0, vars_0 = self.generate_constraints(
            constraints[0], self.result_value_name)
        self.assertEqual(
            const_0, 'sum([model.x[i_0] for i_0 in __import__("numpy")'
            '.where(DATA_FRAME["plants"] == __value)[0]])'
            '<=DATA_FRAME["capacity"][__index]')
        self.assertEqual(
            range_0, 'zip(*__import__("numpy").unique(DATA_FRAME["plants"], '
            'return_index=True))')
        self.assertEqual(vars_0, ["__value", "__index"])

        const_1, range_1, vars_1 = self.generate_constraints(
            constraints[1], self.result_value_name)

        self.assertEqual(
            const_1, 'sum([model.x[i_0] for i_0 in __import__("numpy").'
            'where(DATA_FRAME["markets"] == __value)[0]])>='
            'DATA_FRAME["demand"][__index]')
        self.assertEqual(
            range_1, 'zip(*__import__("numpy").unique(DATA_FRAME["markets"], '
            'return_index=True))')
        self.assertEqual(vars_1, ["__value", "__index"])

        const_2, range_2, vars_2 = self.generate_constraints(
            constraints[2], self.result_value_name)
        self.assertEqual(
            const_2, 'model.x[__index]*100>=DATA_FRAME["demand"][__index]')
        self.assertEqual(range_2, 'model.x')
        self.assertEqual(vars_2, ["__index"])

        model = generate_model_with_data_frame(
            data_frame=self.data_frame,
            variables=self.variables,
            variable_type="NonNegativeIntegers",
            result_value_name=self.result_value_name,
            objective=objective,
            direction="minimize",
            constraints=constraints)
        self.assertTrue(isinstance(model, pyomo_env.ConcreteModel))

        result_x, result_y = solve_model(model, 'baron')
        self.assertTrue(
            np.array_equal(result_x, np.array([99, 1, 31, 59], dtype='int64')))
        self.assertAlmostEqual(result_y, 2581.2)


if __name__ == '__main__':
    unittest.main()
