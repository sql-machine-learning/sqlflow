# coding: utf8
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

import inspect
from functools import wraps


class Requirement(object):
    """
    Requirement: a basic semantic primitive for contracts based verification
                 that supports logic combinations of primitives and readable
                 diagnostic messages.
    """
    def __init__(self, f, diag, isor=False):
        self.func = f
        self.diag = diag
        self._or = isor

    def __or__(self, other):
        if not isinstance(other, Requirement):
            raise TypeError()

        return Requirement(lambda v: self(v) or other(v), " OR ".join(
            (self.diag, other.diag)), True)

    def __and__(self, other):
        if not isinstance(other, Requirement):
            raise TypeError()

        return Requirement(
            lambda v: self(v) and other(v),
            " AND ".join('({})'.format(p.diag) if p._or else p.diag
                         for p in [self, other]))

    def __call__(self, v):
        try:
            return self.func(v)
        except:
            # An exception implies the requirement is not met
            return False


class Type(Requirement):
    """
    Type: a `Requirement` that checks argument types (maybe 1d-nested). For example:
          Type(0) requires the argument to be an integer,
          Type([Int&Positive]) requires the argument to be a list of positive integers.
    """
    def __init__(self, val):
        def deepcheck(v):
            # NOTE: 1. list that's more than 2d degrades to list at the moment
            #       2. only the 1st element of `val` is respected as the requirements
            #          to all elements of `v`
            # TODO(shendiaomo): support checking dict like `Type({String: Int})`?
            if type(val) in (list, tuple) and len(val) and isinstance(
                    val[0], Requirement):
                return (type(v) in (list, tuple)
                        and all(map(lambda i: val[0](i), v)))
            return isinstance(v, type(val))

        if type(val) in (list, tuple) and len(val) and isinstance(
                val[0], Requirement):
            typename = "list[{}]".format(val[0].diag)
        else:
            typename = type(val).__name__
        super().__init__(deepcheck, "TYPE={}".format(typename))


class Between(Requirement):
    """
    Between: a `Requirement` to the range of an argument. Bounds are included (closed interval).
    """
    def __init__(self, lower, upper):
        super().__init__(lambda v: upper >= v >= lower,
                         "BETWEEN ({},{})".format(lower, upper))


class Greater(Requirement):
    """
    Greater: a `Requirement` to the lower bound of an argument. Synonym of the operator `>`.
    """
    def __init__(self, val):
        super().__init__(lambda v: v > val, "'>{}'".format(val))


class GreaterEqual(Requirement):
    """
    GreaterEqual: a `Requirement` to the lower bound of an argument. Synonym of the operator `>=`.
    """
    def __init__(self, val):
        super().__init__(lambda v: v >= val, "'>={}'".format(val))


class Less(Requirement):
    """
    Less: a `Requirement` to the upper bound of an argument. Synonym of the operator `<`.
    """
    def __init__(self, val):
        super().__init__(lambda v: v < val, "'>{}'".format(val))


class LessEqual(Requirement):
    """
    LessEqual: a `Requirement` to the upper bound of an argument. Synonym of the operator `<=`.
    """
    def __init__(self, val):
        super().__init__(lambda v: v <= val, "'<={}'".format(val))


class In(Requirement):
    """
    In: a `Requirement` to the value of an argument. Synonym of the keyword `in`.
    """
    def __init__(self, *args):
        super().__init__(lambda v: v in args,
                         "IN ({})".format(",".join(str(i) for i in args)))


# Shortcuts for requirements that're often used by ML models
Int = Type(1)
Float = Type(1.)
Positive = Greater(0)
Natural = Int & Positive


class Diagnostics(AttributeError):
    """
    Diagnostics: summarizes diagnostic messages from a contract checking.
    """
    def __init__(self):
        super().__init__()
        self._diagnostics = []

    def __setitem__(self, param, diag):
        self._diagnostics.append((param, diag))

    def __bool__(self):
        return len(self._diagnostics) != 0

    def __str__(self):
        lines = ["argument(s) didn't meet parameter requirements"]
        for k, v in self._diagnostics:
            if type(v) == tuple:  # diag, arg, mandatory
                lines.append(
                    '{}: Requirements: "{}, {}", Actual: "{}(TYPE={})"'.format(
                        k, v[0], 'REQUIRED' if v[2] else 'OPTIONAL', v[1],
                        type(v[1]).__name__))
            elif v == 'UNEXPECTED':
                lines.append('{}: {}'.format(k, v))
            else:
                lines.append('{}: Requirements: "{}", Actual: MISSING'.format(
                    k, v + ", REQUIRED" if v else "REQUIRED"))
        return '\n'.join(lines)


def require(**contracts):
    """
    Input contract decorator.

    :param contracts: dict of parameter names to argument `Requirement`s.
    :return: func, decorator function
    """
    def decorator(func):
        @wraps(func)
        def decorated(*args, **kwargs):
            check_requirements(*extract_params_args(func, args, kwargs),
                               contracts)
            return func(*args, **kwargs)

        return decorated

    return decorator


def extract_params_args(func, args, kwargs={}):
    """
    Extract the parameter names of function with its arguments and properties.

    :param func: a function or callable object to be extracted
    :param args: [arg, ...], positional arguments
    :param kwargs: {param: arg, ...}, variadic keyword arguments
    :return: parameters to arguments, required positional parameters and unexpected keyword parameters
             tuple(dict{param: arg, ...}, set{param, ...}, set{param, ...})
    """
    func_params = inspect.signature(func).parameters
    unexpected = kwargs.keys() - func_params.keys()
    for s, p in func_params.items():
        if p.kind == p.VAR_KEYWORD:
            unexpected = {}
    params_args = dict(zip(func_params, args))
    params_args.update(kwargs)
    mandatory = {
        s
        for s, p in func_params.items()
        if p.kind == p.POSITIONAL_OR_KEYWORD and p.default == p.empty
    }
    return params_args, mandatory, unexpected


def check_requirements(params_args, mandatory, unexpected, contracts):
    """
    Check whether `params_args`, `mandatory` and `unexpected` meet the requirements of contracts.

    :param params_args: dict of parameters to arguments
    :param mandatory: set of required positional parameters
    :param unexpected: unexpected keyword parameters
    :param contracts: dict of parameters to requirements (a `Requirement` object)
    :return: parameters to arguments, required positional parameters and unexpected keyword parameters
             tuple(dict{param: arg, ...}, set{param, ...}, set{param, ...})
    :raise: raise Diagnostics if the check failed
    """
    diagnostics = Diagnostics()
    for param, arg in filter(lambda t: t[0] in contracts, params_args.items()):
        # Checking violations
        try:
            if not contracts[param](arg):
                diagnostics[param] = contracts.diag, arg, param in mandatory
        except Exception as e:
            diagnostics[param] = contracts[param].diag, arg, param in mandatory

    for param in mandatory - params_args.keys():
        diagnostics[
            param] = contracts[param].diag if param in contracts else ''
    for param in unexpected:
        diagnostics[param] = "UNEXPECTED"
    if diagnostics:
        raise diagnostics


def check_requirements_for_existed(func, kwargs, **contracts):
    """
    Check whether `kwargs` meet the requirements of `func`'s `contracts`.

    :param func: a function or callable object to be extracted
    :param kwargs: dict of the form {param: arg, ...}, variadic keyword arguments
    :param contracts: variadic keyword arguments of parameter names to `Requirement`s
    :return: parameters to arguments, required positional parameters and unexpected keyword parameters
             tuple(dict{param: arg, ...}, set{param, ...}, set{param, ...})
    :raise: raise Diagnostics if the check failed
    """
    check_requirements(*extract_params_args(func, [], kwargs), contracts)


# This is an example of using `contracts` to design a framework to
# check various callables
class Contracts(object):
    def __init__(self):
        self._contracts = {}

    def add_requirements(self, func, *aliases, **contracts):
        for alias in list(aliases) + [func]:
            self._contracts[alias] = contracts

    def check_requirements(self, func, kwargs):
        if func in self._contracts:
            check_requirements_for_existed(func, kwargs,
                                           **self._contracts[func])


if __name__ == '__main__':
    import tensorflow as tf
    import xgboost as xgb
    my_contracts = Contracts()
    my_contracts.add_requirements(tf.estimator.DNNClassifier,
                                  hidden_units=Type([Natural]),
                                  n_classes=Int & GreaterEqual(2),
                                  optimizer=In('RMSprop', 'Adagrad'),
                                  feature_columns=Type({}))

    my_contracts.add_requirements(xgb.XGBModel,
                                  xgb.XGBClassifier,
                                  learning_rate=Float & Between(0, 1),
                                  booster=In(
                                      "binary:hinge",
                                      "binary:logistic",
                                      "binary:logitraw",
                                      "multi:softmax",
                                      "multi:softprob",
                                      "rank:map",
                                      "rank:ndcg",
                                      "rank:pairwise",
                                      "reg:gamma",
                                      "reg:logistic",
                                  ),
                                  max_depth=Natural)

    model_params = {
        "hidden_units": [0, 1, 2],
        "feature_column": {},
        "n_classes": 0
    }

    # my_contracts.check_requirements(tf.estimator.DNNClassifier, model_params)
    # If we enable the above code line, the error message will look like:

    #     Traceback (most recent call last):
    #       File "contracts.py", line 252, in <module>
    #         dnn_contracts.check_requirements(tf.estimator.DNNClassifier, model_params)
    #       File "contracts.py", line 238, in check_requirements
    #         check_requirements_for_existed(func, kwargs, **self._contracts[func])
    #       File "contracts.py", line 225, in check_requirements_for_existed
    #         check_requirements(*extract_params_args(func, [], kwargs), contracts)
    #       File "contracts.py", line 211, in check_requirements
    #         raise diagnostics
    #     __main__.Diagnostics: argument(s) didn't meet parameter requirements
    #     hidden_units: Requirements: "TYPE=list[TYPE=int AND '>0'], REQUIRED", Actual: "[0, 1, 2](TYPE=list)"
    #     n_classes: Requirements: "TYPE=int AND '>=2', OPTIONAL", Actual: "0(TYPE=int)"
    #     feature_columns: Requirements: "TYPE=dict, REQUIRED", Actual: MISSING
    #     feature_column: UNEXPECTED
