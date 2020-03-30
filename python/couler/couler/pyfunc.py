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
"""This Module provides some helper function for Argo"""

import base64
import importlib.util
import inspect
import os
import re
import textwrap


def _argo_safe_name(name):
    """Some names are to be used in the Argo YAML file. For example,
    the generateName and template name in
    https://github.com/argoproj/argo/blob/master/examples/hello-world.yaml. As
    Argo is to use the YAML as part of Kubernetes job description
    YAML, these names must follow Kubernetes's convention -- no
    period or underscore. This function replaces these prohibited
    characters into dashes.
    """
    if name is None:
        return None
    # '_' and '.' are not allowed
    return re.sub(r"_|\.", "-", name)


def invocation_location():
    """If a function A in file B calls function C, which in turn calls
    invocation_location(), the call returns information about the invocation,
    in particular, the caller's name "A" and the line number where A
    calls C. Return (B + line_number) as function_name if A doesn't exist,
    where users directly calls C in file B.

    :return: a tuple of (function_name, invocation_line)
    """
    stack = inspect.stack()
    if len(stack) < 5:
        line_number = stack[len(stack) - 1][2]
        func_name = "%s-%d" % (
            _argo_safe_name(workflow_name()),
            line_number,
        )
    else:
        func_name = _argo_safe_name(stack[2][3])
        line_number = stack[3][2]
    return func_name, line_number


def body(func_obj):
    """If a function A calls body(), the call returns the Python source code of
    the function definition body (not including the signature) of A.
    """
    if func_obj is None:
        return None
    code = inspect.getsource(func_obj)
    # Remove function signature
    code = code[code.find(":") + 1:]  # noqa: E203
    # Function might be defined in some indented scope
    # (e.g. in another function).
    # We need to handle this and properly dedent the function source code
    return textwrap.dedent(code)


def workflow_name():
    """Return the workflow name that defines the workflow.
    """
    wf_name = os.getenv("workflow_name")
    if wf_name != "":
        return wf_name
    stacks = inspect.stack()
    frame = inspect.stack()[len(stacks) - 1]
    full_path = frame[0].f_code.co_filename
    filename, _ = os.path.splitext(os.path.basename(full_path))
    filename = _argo_safe_name(filename)
    return filename


def input_parameter(function_name, var_pos):
    """Generate parameter name for using as template input parameter names
    in Argo YAML.  For example, the parameter name "message" in the
    container template print-message in
    https://github.com/argoproj/argo/tree/master/examples#output-parameters.
    """
    return "para-%s-%s" % (function_name, var_pos)


def container_output(function_name, caller_line, output_id):
    """Generate output name from an Argo container template.  For example,
    "{{steps.generate-parameter.outputs.parameters.hello-param}}" used in
    https://github.com/argoproj/argo/tree/master/examples#output-parameters.
    """
    function_id = invocation_name(function_name, caller_line)
    return "couler.%s.%s.outputs.parameters.%s" % (
        function_name,
        function_id,
        output_id,
    )


def script_output(function_name, caller_line):
    """Generate output name from an Argo script template.  For example,
    "{{steps.generate.outputs.result}}" in
    https://github.com/argoproj/argo/tree/master/examples#scripts--results
    """
    function_id = invocation_name(function_name, caller_line)
    return "couler.%s.%s.outputs.result" % (function_name, function_id)


def invocation_name(function_name, caller_line):
    """Argo YAML requires that each step, which is an invocation to a
    template, has a name.  For example, hello1, hello2a, and hello2b
    in https://github.com/argoproj/argo/tree/master/examples#steps.
    However, in Python programs, there are no names for function
    invocations.  So we hype a name by the callee and where it is
    called.
    """
    return "%s-%s" % (function_name, caller_line)


def load_cluster_config():
    """Load user provided cluster specification file. For example,
    config file for Sigma EU95 cluster is placed at 'couler/clusters/eu95.py'.
    """
    module_file = os.getenv("couler_cluster_config")
    if module_file is None:
        return None
    spec = importlib.util.spec_from_file_location(module_file, module_file)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)

    return module.cluster


def encode_base64(value):
    """
    Encode a string using base64 and return a binary string.
    This function is used in Secret creation.
    For example, the secrets for Argo YAML:
    https://github.com/argoproj/argo/blob/master/examples/README.md#secrets
    """
    bencode = base64.b64encode(value.encode("utf-8"))
    return str(bencode, "utf-8")


def _is_digit(value):
    if str(value).isdigit():
        return True
    else:
        try:
            float(str(value))
            return True
        except ValueError:
            return False
    return False
