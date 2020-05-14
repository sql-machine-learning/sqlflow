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

import contracts
from contracts import (Between, Diagnostics, Float, In, Int, Positive, Type,
                       require)


class TestContracts(unittest.TestCase):
    def test_type(self):
        # 1. Test the decorator way
        @require(x=Int)
        def f(x):
            pass

        self.assertIsNone(f(0))
        # Type violation
        self.assertRaisesRegex(Diagnostics,
                               r'(?m)^.*TYPE=int.*\(TYPE=float\)"$', f, 0.)
        # Missing required positional argument
        self.assertRaisesRegex(Diagnostics, r'(?m)^.*TYPE=int.*MISSING$', f)
        # Unexpected keyword arguments
        self.assertRaisesRegex(Diagnostics, r'(?m)^y: UNEXPECTED$', f, y=1)
        # Contracts don't handle unexpected positional arguments
        self.assertRaises(TypeError, f, 1, 2)

        # 2. Test the api way (for existed callables that cannot be modified)
        def f(x):
            pass

        def require_f(kwargs):
            contracts.check_requirements_for_existed(f, kwargs, x=Int)

        # Do the same thing as the first part
        self.assertIsNone(require_f(kwargs={"x": 0}))
        # Type violation
        self.assertRaisesRegex(Diagnostics,
                               r'(?m)^.*TYPE=int.*\(TYPE=float\)"$', require_f,
                               {"x": 0.})
        # Missing required positional argument
        self.assertRaisesRegex(Diagnostics, r'(?m)^.*TYPE=int.*MISSING$',
                               require_f, {})
        # Unexpected keyword arguments
        self.assertRaisesRegex(Diagnostics, r'(?m)^y: UNEXPECTED$', require_f,
                               {"y": 1})

    def test_combination(self):
        @require(x=(Int | Float | Type("")) & (Between(0, 1) | In('relu')))
        def f(x):
            '''
            param x: 1. int or float in the closed interval [0, 1], or
                     2. the string 'relu'
            '''
            pass

        self.assertIsNone(f(0))
        self.assertIsNone(f(1))
        self.assertIsNone(f(0.))
        self.assertIsNone(f(1.))
        self.assertIsNone(f(0.5))
        self.assertRaises(Diagnostics, f, 2)
        self.assertRaises(Diagnostics, f, -1e-10)
        self.assertRaises(Diagnostics, f, '2')
        self.assertIsNone(f('relu'))

    def test_1d_list(self):
        @require(x=Type([Float & Positive]))  # list of positive floats
        def f(x):
            pass

        self.assertRaisesRegex(
            Diagnostics,
            r'''(?m)^.*TYPE=list\[TYPE=float AND '>0'\], REQUIRED.*$''', f, 0.)
        self.assertRaisesRegex(
            Diagnostics,
            r'''(?m)^.*TYPE=list\[TYPE=float AND '>0'\], REQUIRED.*$''', f,
            [0, 0.])
        self.assertRaisesRegex(
            Diagnostics,
            r'''(?m)^.*TYPE=list\[TYPE=float AND '>0'\], REQUIRED.*$''', f,
            [0., 1])
        self.assertIsNone(f([1e-10, 1e10]))

    def test_2d_list(self):
        @require(x=Type([[Float & Positive]])
                 )  # list of list of positive floats
        def f(x):
            pass

        self.assertRaisesRegex(Diagnostics,
                               r'''(?m)^.*TYPE=list, REQUIRED.*$''', f, 0.)
        # NOTE: list that's more than 2d degrades to list at the moment
        self.assertIsNone(f([0, 0.]))
        self.assertIsNone(f([0., 1]))
        self.assertIsNone(f([1e-10, 1e10]))
