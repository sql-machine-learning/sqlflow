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

__all__ = [
    'generate_unique_result_value_name',
    'generate_objective_and_constraint_expression',
]


def generate_unique_result_value_name(columns, result_value_name, variables):
    variables = [v.lower() for v in variables]
    columns_lower = [c.lower() for c in columns]
    for v in variables:
        assert v in columns_lower, "cannot find variable %s column" % v

    assert len(set(variables)) == len(variables), \
        "duplicate variables are not allowed"

    if len(variables) > 1 or result_value_name.lower() != variables[0]:
        assert result_value_name.lower() not in columns_lower, \
            "result value name should not be duplicated with the column name"
        return result_value_name

    result_value_name += "_value"
    if result_value_name.lower() not in columns_lower:
        return result_value_name

    i = 0
    while True:
        new_name = result_value_name + ("_%d" % i)
        i += 1
        if new_name.lower() not in columns_lower:
            return new_name


def update_by_column_names(columns, tokens, variables, result_value_name,
                           group_by):
    tokens = list(copy.copy(tokens))
    variables = list(copy.copy(variables))

    def to_column_name_or_return_itself(name):
        for c in columns:
            if name.lower() == c.lower():
                return c, True
        return name, False

    for i, var in enumerate(variables):
        new_var, ok = to_column_name_or_return_itself(var)
        if not ok:
            raise ValueError("cannot find column %s in table" % var)
        variables[i] = new_var

    result_value_name, _ = to_column_name_or_return_itself(result_value_name)

    new_result_value_name = generate_unique_result_value_name(
        columns=columns,
        result_value_name=result_value_name,
        variables=variables)

    tokens = list(tokens)
    for i, token in enumerate(tokens):
        new_token, _ = to_column_name_or_return_itself(token)
        if new_token == result_value_name:
            new_token = new_result_value_name

        tokens[i] = new_token

    if group_by:
        group_by, ok = to_column_name_or_return_itself(group_by)
        if not ok:
            raise ValueError("cannot find GROUP BY column %s in table" %
                             group_by)

    return tokens, variables, new_result_value_name, group_by


def try_convert_to_aggregation_function(token):
    AGGREGATION_FUNCTIONS = {
        'SUM': 'sum',
    }
    return AGGREGATION_FUNCTIONS.get(token, None)


def try_convert_comparision_token(token):
    COMPARISION_TOKENS = {
        '=': '==',
    }
    return COMPARISION_TOKENS.get(token, None)


def generate_group_by_range_and_index_str(group_by, data_str, value_str,
                                          index_str):
    numpy_str = '__import__("numpy")'
    group_by_data_str = '%s["%s"]' % (data_str, group_by)
    outer_range_str = 'zip(*%s.unique(%s, return_index=True))' % (
        numpy_str, group_by_data_str)
    inner_range_str = '%s.where(%s == %s)[0]' % (numpy_str, group_by_data_str,
                                                 value_str)
    return outer_range_str, inner_range_str, [value_str, index_str]


def find_prev_non_blank_token(tokens, i):
    if i < 0 or i >= len(tokens):
        return None

    while i >= 0:
        if tokens[i].strip():
            return tokens[i]

        i -= 1

    return None


def find_matched_aggregation_function_brackets(tokens, i):
    left_bracket_num = 0
    left_bracket_indices = []
    right_bracket_indices = []
    while i < len(tokens):
        if tokens[i] == '(':
            left_bracket_indices.append(i)
            right_bracket_indices.append(-1)
            left_bracket_num += 1
        elif tokens[i] == ')':
            if left_bracket_num <= 0:
                raise ValueError("bracket not match")
            left_bracket_num -= 1
            right_bracket_indices[left_bracket_num] = i
            if left_bracket_num == 0:
                i += 1
                break

        i += 1

    if left_bracket_num != 0:
        raise ValueError("bracket not match")

    agg_left_bracket_indices = []
    agg_right_bracket_indices = []
    for left_idx, right_idx in zip(left_bracket_indices,
                                   right_bracket_indices):
        token = find_prev_non_blank_token(tokens, left_idx - 1)
        if try_convert_to_aggregation_function(token):
            agg_left_bracket_indices.append(left_idx)
            agg_right_bracket_indices.append(right_idx)

    i = min(i, len(tokens))
    return agg_left_bracket_indices, agg_right_bracket_indices, i


def get_bracket_depth(idx, left_bracket_indices, right_bracket_indices):
    depth = -1
    for i, left_idx in enumerate(left_bracket_indices):
        if idx >= left_idx and idx <= right_bracket_indices[i]:
            depth += 1

    if depth < 0:
        raise ValueError("cannot find bracket depth")

    return depth


def generate_token_in_non_aggregation_expression(token, columns,
                                                 result_value_name, group_by,
                                                 data_str, index_str):
    if try_convert_to_aggregation_function(token):
        return try_convert_to_aggregation_function(token)

    if try_convert_comparision_token(token):
        return try_convert_comparision_token(token)

    if token == result_value_name:
        raise ValueError(
            "invalid expression: result variable %s should not occur in the non-aggregation part of objective or constraint",
            result_value_name)

    if token in columns:
        if not group_by:
            raise ValueError(
                "invalid expression: column %s should not occur in the non-aggregation part of objective or constraint without GROUP BY",
                token)
        return '%s["%s"][%s]' % (data_str, token, index_str)

    return token


def generate_token_in_aggregation_expression(token, columns, result_value_name,
                                             variable_str, data_str, depth):
    if try_convert_to_aggregation_function(token):
        return try_convert_to_aggregation_function(token)

    if try_convert_comparision_token(token):
        return try_convert_comparision_token(token)

    if token == result_value_name:
        return '%s[i_%d]' % (variable_str, depth)

    if token in columns:
        return '%s["%s"][i_%d]' % (data_str, token, depth)

    return token


def generate_non_aggregation_constraint_expression(tokens, columns,
                                                   result_value_name,
                                                   variable_str, data_str,
                                                   index_str):
    result_tokens = []
    for token in tokens:
        if try_convert_comparision_token(token):
            result_tokens.append(try_convert_comparision_token(token))
            continue

        if token == result_value_name:
            result_tokens.append("%s[%s]" % (variable_str, index_str))
            continue

        if token in columns:
            result_tokens.append('%s["%s"][%s]' % (data_str, token, index_str))
            continue

        result_tokens.append(token)

    return "".join(result_tokens), variable_str, [index_str]


def generate_objective_or_aggregated_constraint_expression(
    tokens, group_by, result_value_name, columns, variable_str, data_str,
    value_str, index_str):
    iter_vars = None
    outer_range = None
    inner_range = None
    if group_by:
        outer_range, inner_range, iter_vars = generate_group_by_range_and_index_str(
            group_by=group_by,
            data_str=data_str,
            value_str=value_str,
            index_str=index_str)

    idx = 0
    result_tokens = []
    while idx < len(tokens):
        left_bracket_indices, right_bracket_indices, next_idx = find_matched_aggregation_function_brackets(
            tokens, idx)
        if left_bracket_indices:
            left_bracket_idx = left_bracket_indices[0]
            right_bracket_idx = right_bracket_indices[0]
        else:
            left_bracket_idx = next_idx
            right_bracket_idx = next_idx

        while idx < left_bracket_idx:
            token = generate_token_in_non_aggregation_expression(
                token=tokens[idx],
                columns=columns,
                result_value_name=result_value_name,
                group_by=group_by,
                data_str=data_str,
                index_str=index_str)
            result_tokens.append(token)
            idx += 1

        if left_bracket_idx == right_bracket_idx:
            continue

        while idx <= right_bracket_idx:
            depth = get_bracket_depth(idx, left_bracket_indices,
                                      right_bracket_indices)

            if tokens[idx] == "(":
                result_tokens.append(tokens[idx])
                if idx in left_bracket_indices:
                    result_tokens.append("[")
            elif tokens[idx] == ")":
                if idx in right_bracket_indices:
                    result_tokens.append(" ")
                    if group_by:
                        for_range = "for i_%d in %s" % (depth, inner_range)
                    else:
                        for_range = "for i_%d in %s" % (depth, variable_str)
                    result_tokens.append(for_range)
                    result_tokens.append("]")

                result_tokens.append(tokens[idx])
            else:
                token = generate_token_in_aggregation_expression(
                    token=tokens[idx],
                    columns=columns,
                    result_value_name=result_value_name,
                    variable_str=variable_str,
                    data_str=data_str,
                    depth=depth)
                result_tokens.append(token)

            idx += 1

        while idx < next_idx:
            token = generate_token_in_non_aggregation_expression(
                token=tokens[idx],
                columns=columns,
                result_value_name=result_value_name,
                group_by=group_by,
                data_str=data_str,
                index_str=index_str)
            result_tokens.append(token)
            idx += 1

    return "".join(result_tokens), outer_range, iter_vars


def generate_objective_or_constraint_expression(columns, tokens, variables,
                                                result_value_name, group_by,
                                                variable_str, data_str,
                                                value_str, index_str):
    tokens, variables, result_value_name, group_by = update_by_column_names(
        columns=columns,
        tokens=tokens,
        variables=variables,
        result_value_name=result_value_name,
        group_by=group_by)

    has_aggregation_func = False
    for token in tokens:
        if try_convert_to_aggregation_function(token):
            has_aggregation_func = True
            break

    if has_aggregation_func:
        expr, for_range, iter_vars = generate_objective_or_aggregated_constraint_expression(
            tokens=tokens,
            group_by=group_by,
            result_value_name=result_value_name,
            columns=columns,
            variable_str=variable_str,
            data_str=data_str,
            value_str=value_str,
            index_str=index_str)
    else:
        if group_by:
            raise ValueError(
                "GROUP BY must be used with aggregation function like SUM together"
            )

        expr, for_range, iter_vars = generate_non_aggregation_constraint_expression(
            tokens=tokens,
            columns=columns,
            result_value_name=result_value_name,
            variable_str=variable_str,
            data_str=data_str,
            index_str=index_str)

    return expr, for_range, iter_vars


def generate_objective_and_constraint_expression(columns,
                                                 objective,
                                                 constraints,
                                                 variables,
                                                 result_value_name,
                                                 variable_str,
                                                 data_str,
                                                 value_str="__value",
                                                 index_str="__index"):
    obj_expr = ""
    constraint_exprs = []

    if objective:
        obj_expr, for_range, iter_vars = generate_objective_or_constraint_expression(
            columns=columns,
            tokens=objective,
            variables=variables,
            result_value_name=result_value_name,
            group_by="",
            variable_str=variable_str,
            data_str=data_str,
            value_str=value_str,
            index_str=index_str)
        assert for_range is None and iter_vars is None, \
            "invalid objective expression"

    if constraints:
        for c in constraints:
            tokens = c.get("tokens")
            group_by = c.get("group_by")

            expr, for_range, iter_vars = generate_objective_or_constraint_expression(
                columns=columns,
                tokens=tokens,
                variables=variables,
                result_value_name=result_value_name,
                group_by=group_by,
                variable_str=variable_str,
                data_str=data_str,
                value_str=value_str,
                index_str=index_str)

            if iter_vars:
                assert for_range, "both for_range and iter_vars must be None"
            else:
                assert not for_range, "both for_range and iter_vars must be not None"

            constraint_exprs.append((expr, for_range, iter_vars))

    return obj_expr, constraint_exprs
