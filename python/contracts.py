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
from functools import reduce, wraps


class Requirement(object):
    """
    Requirement: a basic semantic primitive for contracts based verification
                 that supports logic combinations of primitives and readable
                 diagnostic messages.
    """
    def __init__(self, f, desc="", isor=False, issubj=False, typeset=None):
        self.func = f
        self._desc = desc
        self._or = isor
        self._dnf = [[self]]
        self._typeset = typeset
        self._issubj = issubj
        self._custom_diag = False

    def __or__(self, other):
        if not isinstance(other, Requirement):
            raise TypeError()

        ret = Requirement(lambda v: self(v) or other(v), "", isor=True)
        ret._dnf = self._dnf + other._dnf
        return ret

    def __and__(self, other):
        if not isinstance(other, Requirement):
            raise TypeError()
        if self._typeset and other._typeset:
            typeset = self._typeset & other._typeset
            if not typeset:
                raise TypeError(
                    f"type can not be both in {self._typeset} and {other._typeset}"
                )
        ret = Requirement(lambda v: self(v) and other(v))
        ret._dnf = []
        for i in self._dnf:
            for j in other._dnf:
                cnf = i + j
                type_cnf = list(filter(None, map(lambda r: r._typeset, cnf)))
                if type_cnf:
                    typeset = reduce(lambda l, r: l & r, type_cnf)
                    if not typeset:
                        # Eliminate the cnf because it's always False
                        continue
                ret._dnf.append(cnf)
        return ret

    def __call__(self, v):
        for ands in self._dnf:
            r = ands[0]
            res = r.func(v)
            for r in ands[1:]:
                res = res and r.func(v)
            if res:
                return True
        return False

    def __str__(self):
        if self._custom_diag:
            return self._desc
        messages = []
        uniq_cnfs = []
        for ands in sorted(self._dnf, key=lambda a: len(a)):
            ands.sort(key=lambda r: r._issubj, reverse=True)
            r = ands[0]
            desc, hastype = r._desc, r._typeset
            uniq_ands = set([desc])
            for r in ands[1:]:
                if r._desc in uniq_ands:
                    continue
                uniq_ands.add(r._desc)
                if hastype:
                    hastype = False
                    desc = desc + " that's " + r._desc
                else:
                    desc = desc + " and " + r._desc
            if not any(map(lambda s: s.issubset(uniq_ands), uniq_cnfs)):
                # Eliminate the cnf because it's redundant
                uniq_cnfs.append(uniq_ands)
                messages.append(desc)
        return ", or ".join(messages)

    def desc(self, desc):
        self._desc = desc
        self._dnf = [[self]]
        self._custom_desc = True
        return self


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
            typename = "`list of {}`".format(val[0])
        else:
            typename = type(val).__name__
        super().__init__(deepcheck,
                         "{}".format(typename),
                         issubj=True,
                         typeset={type(val)})


class Between(Requirement):
    """
    Between: a `Requirement` to the range of an argument. Bounds are included (closed interval).
    """
    def __init__(self, lower, upper):
        assert upper >= lower
        assert isinstance(upper, (int, float))
        super().__init__(lambda v: upper >= v >= lower,
                         "BETWEEN ({},{})".format(lower, upper),
                         typeset={int, float})


class Greater(Requirement):
    """
    Greater: a `Requirement` to the lower bound of an argument. Synonym of the operator `>`.
    """
    def __init__(self, val):
        assert isinstance(val, (int, float))
        super().__init__(lambda v: v > val,
                         "> {}".format(val),
                         typeset={int, float})


class GreaterEqual(Requirement):
    """
    GreaterEqual: a `Requirement` to the lower bound of an argument. Synonym of the operator `>=`.
    """
    def __init__(self, val):
        super().__init__(lambda v: v >= val,
                         ">= {}".format(val),
                         typeset={int, float})


class Less(Requirement):
    """
    Less: a `Requirement` to the upper bound of an argument. Synonym of the operator `<`.
    """
    def __init__(self, val):
        super().__init__(lambda v: v < val,
                         "< {}".format(val),
                         typeset={int, float})


class LessEqual(Requirement):
    """
    LessEqual: a `Requirement` to the upper bound of an argument. Synonym of the operator `<=`.
    """
    def __init__(self, val):
        super().__init__(lambda v: v <= val,
                         "<= {}".format(val),
                         typset={int, float})


class In(Requirement):
    """
    In: a `Requirement` to the value of an argument. Synonym of the keyword `in`.
    """
    def __init__(self, *args):
        if all(map(lambda v: isinstance(v, (int, float)), args)):
            typeset = {int, float}
        else:
            typeset = set(map(type, args))
            assert len(typeset) == 1
        super().__init__(lambda v: v in args,
                         "IN ({})".format(",".join(str(i) for i in args)),
                         typeset=typeset)


# Shortcuts for requirements that're often used by ML models
Int = Type(1)
Float = Type(1.)
String = Type('')
Positive = Greater(0) & (Int | Float)
PositiveInt = Int & Greater(0)


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
                arg_text = '"{}(string)"'.format(v[1]) if type(
                    v[1]) == str else v[1]
                lines.append('{}({}) must be {}. Actual: {}'.format(
                    k, 'required' if v[2] else 'optional', v[0], arg_text))
            elif v == 'UNEXPECTED':
                lines.append('{} is {}'.format(k, v))
            else:
                lines.append('{} is required but is MISSING. {}'.format(
                    k, "Should be " + v if v else ""))
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
        if not contracts[param](arg):
            diagnostics[param] = str(contracts[param]), arg, param in mandatory
    for param in mandatory - params_args.keys():
        # Checking missing arguments
        diagnostics[param] = str(
            contracts[param]) if param in contracts else ''
    for param in unexpected:
        # Checking unexpected arguments
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
                                  hidden_units=Type([PositiveInt]),
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
                                  max_depth=PositiveInt)

    model_params = {
        "hidden_units": [0, 1, 2],
        "feature_column": {},
        "n_classes": 0
    }

    # my_contracts.check_requirements(tf.estimator.DNNClassifier, model_params)
    # If we enable the above code line, the error message will look like:
    #     Traceback (most recent call last):
    #         ...
    #         raise diagnostics
    #     __main__.Diagnostics: argument(s) didn't meet parameter requirements

    #     hidden_units(required) must be `list of int that's > 0`. Actual: [0, 1, 2]
    #     n_classes(optional) must be int that's >= 2. Actual: 0
    #     feature_columns is required but is MISSING. Should be dict
    #     feature_column is UNEXPECTED
