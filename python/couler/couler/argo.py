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

import atexit
import copy
import types
import uuid
from collections import OrderedDict

import couler.pyfunc as pyfunc
import pyaml

_wf = dict()
_secrets = dict()
_steps = OrderedDict()
_templates = dict()
_update_steps_lock = True
_run_concurrent_lock = False
_concurrent_func_line = -1
# We need to fetch the name before triggering atexit, as the atexit handlers
# cannot get the original Python filename.
_wf_name = pyfunc.workflow_name()
# '_when_prefix' represents 'when' prefix in Argo YAML. For example,
# https://github.com/argoproj/argo/blob/master/examples/README.md#conditionals
_when_prefix = None
# '_condition_id' records the line number where the 'couler.when()' is invoked.
_condition_id = None
# '_while_steps' records the step of recursive logic
_while_steps = OrderedDict()
# '_while_lock' indicates the recursive call starts
_while_lock = False
# TTL_cleaned for the workflow
_workflow_ttl_cleaned = None

_cluster_config = pyfunc.load_cluster_config()


class Artifact(object):
    def __init__(self, id, path, type=None):
        self.id = id
        self.path = path
        self.type = type


def _update_steps(function_name, caller_line, args=None, template_name=None):
    """
    Step of Argo Yaml contains name, related template and parameters.
    thus, we insert these information into couler.step.
    """
    function_id = pyfunc.invocation_name(function_name, caller_line)

    # Update `steps` only if needed
    if _update_steps_lock:
        step_template = OrderedDict()
        if _run_concurrent_lock:
            _id = pyfunc.invocation_name(template_name, caller_line)
            step_template["name"] = _id
        else:
            step_template["name"] = function_id

        if template_name is None:
            step_template["template"] = function_name
        else:
            step_template["template"] = template_name

        if _when_prefix is not None:
            step_template["when"] = _when_prefix

        if args is not None:
            parameters = []
            for i in range(len(args)):
                value = args[i]
                if "couler" in value:
                    tmp = args[i].split(".")
                    if len(tmp) < 3:
                        raise ValueError(
                            "wrong container return representation")
                    # To avoid duplicate map function
                    # value = ".".join(map(str, tmp[2:]))
                    value = tmp[2]
                    for item in tmp[3:]:
                        value = value + "." + item
                    value = '"{{steps.%s}}"' % value

                if _run_concurrent_lock:
                    parameters.append({
                        "name":
                        pyfunc.input_parameter(template_name, i),
                        "value":
                        value,
                    })
                else:
                    parameters.append({
                        "name":
                        pyfunc.input_parameter(function_name, i),
                        "value":
                        value,
                    })
            step_template["arguments"] = {"parameters": parameters}

        if _condition_id is not None:
            function_id = _condition_id

        if _while_lock:
            if function_id in _while_steps:
                _while_steps.get(function_id).append(step_template)
            else:
                _while_steps[function_id] = [step_template]
        else:
            if function_id in _steps:
                _steps.get(function_id).append(step_template)
            else:
                _steps[function_id] = [step_template]


def run_script(image, command=None, source=None, env=None, resources=None):
    """Generate an Argo script template.  For example,
    https://github.com/argoproj/argo/tree/master/examples#scripts--results
    """
    function_name, caller_line = pyfunc.invocation_location()

    if function_name not in _templates:
        template = OrderedDict({"name": function_name})
        # Script
        if source is not None:
            script = OrderedDict()
            if image is not None:
                script["image"] = image

            if command is None:
                command = "python"
            script["command"] = [command]

            # To retrieve function code
            script["source"] = (pyfunc.body(source)
                                if command.lower() == "python" else source)

            if env is not None:
                script["env"] = _convert_dict_to_env_list(env)

            if resources is not None:
                script["resources"] = _resources(resources)

            template["script"] = script
        else:
            raise ValueError("Input script can not be null")

        # config the pod with cluster specific config
        template = _update_pod_config(template)

        _templates[function_name] = template

    _update_steps(function_name, caller_line)
    return pyfunc.script_output(function_name, caller_line)


def run_container(
    image,
    command=None,
    args=None,
    output=None,
    env=None,
    secret=None,
    resources=None,
):
    """Generate an Argo container template.  For example, the template whalesay
    in https://github.com/argoproj/argo/tree/master/examples#hello-world
    """
    function_name, caller_line = pyfunc.invocation_location()
    output_id = None

    if function_name not in _templates:
        template = OrderedDict({"name": function_name})

        # Generate the inputs parameter for the template
        if args is not None:
            parameters = []
            for i in range(len(args)):
                para_name = pyfunc.input_parameter(function_name, i)
                parameters.append({"name": para_name})

            inputs = OrderedDict({"parameters": parameters})
            template["inputs"] = inputs

        # Generate the container template
        container = OrderedDict()
        if image is not None:
            container["image"] = image

        container["command"] = ["bash", "-c"]
        if isinstance(command, list):
            container["command"].extend(command)
        elif command is not None:
            container["command"].extend([command])

        if args is not None:
            # Rewrite the args into yaml format
            container["args"] = []
            for i in range(len(args)):
                para_name = pyfunc.input_parameter(function_name, i)
                arg_yaml = '"{{inputs.parameters.%s}}"' % para_name
                container["args"].append(arg_yaml)

        if env is not None:
            container["env"] = _convert_dict_to_env_list(env)

        if secret is not None:
            env_secrets = _convert_secret_to_list(secret)
            if "env" not in container.keys():
                container["env"] = env_secrets
            else:
                container["env"].extend(env_secrets)

        if resources is not None:
            container["resources"] = _resources(resources)

        template["container"] = container

        # Generate the output
        if output is not None and isinstance(output, Artifact):
            output_id = output.id
            path = output.path
            _output = OrderedDict()
            _output["parameters"] = [{
                "name": output_id,
                "valueFrom": {
                    "path": path
                }
            }]
            template["outputs"] = _output
        # else TODO, when container does not output anything

        # Update the pod with cluster specific config
        template = _update_pod_config(template)

        _templates[function_name] = template

    if _run_concurrent_lock:
        _update_steps("concurrent_func_name", _concurrent_func_line, args,
                      function_name)
    else:
        _update_steps(function_name, caller_line, args)

    if output_id is None:
        output_id = "output-id-%s" % caller_line

    return pyfunc.container_output(function_name, caller_line, output_id)


def run_job(manifest, success_condition, failure_condition):
    """
    Create a k8s job. For example, the pi-tmpl template in
    https://github.com/argoproj/argo/blob/master/examples/k8s-jobs.yaml
    :param manifest: YAML specification of the job to be created.
    :param success_condition: expression for verifying job success.
    :param failure_condition: expression for verifying job failure.
    :return: output
    """
    if manifest is None:
        raise ValueError("Input manifest can not be null")

    func_name, caller_line = pyfunc.invocation_location()

    if func_name not in _templates:
        template = OrderedDict()
        template["name"] = func_name
        template["resource"] = _create_job(
            manifest=manifest,
            success_condition=success_condition,
            failure_condition=failure_condition,
        )
        # TODO: add input support

        _templates[func_name] = template

    _update_steps(func_name, caller_line)
    # TODO: add output support
    return None


def _extract_step_return(step_output):
    """Extract information for run container or script output
    :param step_output: normal variable string or the Couler input string
    :return: the dict for step information
    """
    ret = {}
    # In case user input a normal variable
    if "couler" not in step_output:
        ret["value"] = step_output
        return ret
    else:
        tmp = step_output.split(".")
        if len(tmp) < 5:
            raise ValueError("Incorrect container return representation")
        name = tmp[1]
        function_id = tmp[2]
        # To avoid duplicate map function
        output = tmp[3]
        for item in tmp[4:]:
            output = output + "." + item

        ret = {"name": name, "id": function_id, "output": output}
        return ret


def when(condition, func1):
    """Generates an Argo conditional step.
    For example, the coinflip example in
    https://github.com/argoproj/argo/blob/master/examples/coinflip.yaml.
    """
    pre = condition["pre"]
    post = condition["post"]
    condition_suffix = condition["condition"]

    pre_dict = _extract_step_return(pre)
    post_dict = _extract_step_return(post)

    step1 = {}
    if "name" in pre_dict:
        left_function_id = pre_dict["id"]
        if left_function_id not in _steps:
            step1["name"] = pre_dict["id"]
            step1["template"] = pre_dict["name"]
            _steps.append([step1])
    else:
        # TODO: fixed if left branch is a variable rather than function
        pre_dict["value"]

    global _when_prefix, _condition_id
    _when_prefix = "{{steps.%s.%s}} %s %s" % (
        pre_dict["id"],
        pre_dict["output"],
        condition_suffix,
        post_dict["value"],
    )
    _condition_id = "%s.%s" % (pre_dict["id"], pre_dict["output"])

    # Enforce the function to run and lock to add into step
    if isinstance(func1, types.FunctionType):
        branch = func1()
        if branch is None:
            raise SyntaxError("require function return value")
    else:
        raise TypeError("condition to run would be a function")

    _when_prefix = None
    _condition_id = None

    return _templates


def exec_while(condition, inner_while):
    """
    Generate the Argo recursive logic. For example
    https://github.com/argoproj/argo/blob/master/examples/README.md#recursion
    """
    # _while_lock means 'exec_while' operation begins to work
    # _while_steps stores logic steps inside the recursion logic
    global _while_lock, _while_steps
    _while_lock = True

    # Enforce inner function of the while-loop to run
    if isinstance(inner_while, types.FunctionType):
        branch = inner_while()
        if branch is None:
            raise SyntaxError("require function return value")
    else:
        raise TypeError("condition to run would be a function")

    branch_dict = _extract_step_return(branch)
    recursive_name = "exec-while-" + branch_dict["name"]
    recursive_id = "exec-while-" + branch_dict["id"]
    if recursive_name not in _templates:
        template = OrderedDict({"name": recursive_name})
    else:
        raise SyntaxError("Recursive function can not be called twice ")

    # Generates leaving point for recursive
    step_out_name = "%s-%s" % (recursive_name, "exit")
    pre = condition["pre"]
    pre_dict = _extract_step_return(pre)
    condition_suffix = condition["condition"]

    # Generates the recursive go to step
    when_prefix = "{{steps.%s.%s}} %s %s" % (
        branch_dict["id"],
        branch_dict["output"],
        condition_suffix,
        pre_dict["value"],
    )
    step_out_template = OrderedDict({
        "name": step_out_name,
        "template": recursive_name,
        "when": when_prefix,
    })
    step_out_id = pyfunc.invocation_name(step_out_name, recursive_id)
    _while_steps[step_out_id] = [step_out_template]

    # Adds steps inside the recursive logic to recursive template
    template["steps"] = list(_while_steps.values())

    # Adds this recursive logic to the _templates
    _templates[recursive_name] = template

    # Add recursive logic to global _steps
    recursive_out_template = OrderedDict({
        "name": recursive_id,
        "template": recursive_name
    })

    if recursive_id in _steps:
        _steps.get(recursive_id).append(recursive_out_template)
    else:
        _steps[recursive_id] = [recursive_out_template]

    _while_lock = False
    _while_steps = OrderedDict()

    return _templates


def map(function, input_list):
    """
    map operation of Couler
    """
    # Enforce the function to run and lock to add into step
    if isinstance(function, types.FunctionType):
        global _update_steps_lock
        _update_steps_lock = False
        para = input_list[0]
        inner = function(para)
        if inner is None:
            raise SyntaxError("require function return value")
        _update_steps_lock = True
    else:
        raise TypeError("require loop over a function to run")

    inner_dict = _extract_step_return(inner)
    inner_step = OrderedDict()
    inner_step["name"] = inner_dict["id"]
    inner_step["template"] = inner_dict["name"]

    parameters = []
    function_template = _templates[inner_dict["name"]]
    input_parameters = function_template["inputs"]["parameters"]

    for para_name in input_parameters:
        parameters.append({
            "name": para_name["name"],
            "value": '"{{item.%s}}"' % para_name["name"],
        })

    inner_step["arguments"] = {"parameters": parameters}

    with_items = []
    inner_step["withItems"] = []

    for para_values in input_list:
        item = {}
        if not isinstance(para_values, list):
            para_values = [para_values]

        for j in range(len(input_parameters)):
            para_name = input_parameters[j]["name"]
            item[para_name] = para_values[j]

        with_items.append(item)

    inner_step["withItems"] = with_items
    _steps[inner_dict["id"]] = [inner_step]

    return inner_step


def concurrent(function_list):
    """
    Start different jobs at the same time
    """
    if not isinstance(function_list, list):
        raise SyntaxError("require input functions as list")

    _, con_caller_line = pyfunc.invocation_location()

    global _concurrent_func_line
    _concurrent_func_line = con_caller_line

    global _run_concurrent_lock
    _run_concurrent_lock = True

    for function in function_list:
        if isinstance(function, types.FunctionType):
            function()
        else:
            raise TypeError("require loop over a function to run")

    _run_concurrent_lock = False


def __dump_yaml():
    wf = copy.deepcopy(_wf)
    wf["apiVersion"] = "argoproj.io/v1alpha1"
    wf["kind"] = "Workflow"
    wf["metadata"] = {"generateName": "%s-" % _wf_name}

    entrypoint = wf["metadata"]["generateName"][:-1]
    ts = [{"name": entrypoint, "steps": list(_steps.values())}]
    ts.extend(_templates.values())

    spec = {}
    spec = _update_workflow_spec(spec)
    spec.update({"entrypoint": entrypoint, "templates": ts})
    wf["spec"] = spec

    if _workflow_ttl_cleaned is not None:
        wf["spec"]["ttlSecondsAfterFinished"] = _workflow_ttl_cleaned
    return wf


def _dump_yaml():
    yaml_str = ""
    if len(_secrets) > 0:
        yaml_str = pyaml.dump(_secrets)
        yaml_str = "%s\n---\n" % yaml_str
    if len(_steps) > 0:
        yaml_str = yaml_str + pyaml.dump(__dump_yaml())
    print(yaml_str)


def artifact(path):
    """
    configure the output object
    """
    _, caller_line = pyfunc.invocation_location()

    # TODO: support outputs to an artifact repo later
    ret = Artifact(path=path,
                   id="output-id-%s" % caller_line,
                   type="parameters")

    return ret


def _predicate(pre, post, condition):
    """Generates an Argo predicate.
    """
    dict_config = {}
    if isinstance(pre, types.FunctionType):
        dict_config["pre"] = pre()
    else:
        dict_config["pre"] = pre

    if isinstance(post, types.FunctionType):
        dict_config["post"] = post()
    else:
        dict_config["post"] = post

    # TODO: check the condition
    dict_config["condition"] = condition

    return dict_config


def equal(pre, post=None):
    if post is not None:
        return _predicate(pre, post, "==")
    else:
        return _predicate(pre, None, "==")


def not_equal(pre, post=None):
    if post is not None:
        return _predicate(pre, post, "!=")
    else:
        return _predicate(pre, None, "!=")


def bigger(pre, post=None):
    if post is not None:
        return _predicate(pre, post, ">")
    else:
        return _predicate(pre, None, ">")


def smaller(pre, post=None):
    if post is not None:
        return _predicate(pre, post, "<")
    else:
        return _predicate(pre, None, "<")


def bigger_equal(pre, post=None):
    if post is not None:
        return _predicate(pre, post, ">=")
    else:
        return _predicate(pre, None, ">=")


def smaller_equal(pre, post=None):
    if post is not None:
        return _predicate(pre, post, "<=")
    else:
        return _predicate(pre, None, "<=")


def _cleanup():
    """Cleanup the cached fields, just used for unit test.
    """
    global _wf, _secrets, _steps, _templates, _update_steps_lock
    _wf = {}
    _secrets = {}
    _steps = OrderedDict()
    _templates = {}
    _update_steps_lock = True


def _convert_dict_to_list(d):
    """This is to convert a Python dictionary to a list, where
    each list item is a dict with `name` and `value` keys.
    """
    if not isinstance(d, dict):
        raise TypeError("The input parameter `d` is not a dict.")

    env_list = []
    for k, v in d.items():
        env_list.append({"name": str(k), "value": str(v)})
    return env_list


def _convert_dict_to_env_list(d):
    """This is to convert a Python dictionary to a list, where
    each list item is a dict with `name` and `value` keys.
    """
    if not isinstance(d, dict):
        raise TypeError("The input parameter `d` is not a dict.")

    env_list = []
    for k, v in d.items():
        if k == "secrets":
            if not isinstance(v, list):
                raise TypeError("The environment secrets should be a list.")
            for s in v:
                env_list.append(s)
        else:
            env_list.append({"name": str(k), "value": "'%s'" % str(v)})
    return env_list


def _convert_secret_to_list(secret):
    """Convert a couler.Secret object to a list, where
    each list item is a dict with `name` and `valueFrom` keys.
    """

    if not isinstance(secret, Secret):
        raise TypeError("The input parameter `secret` is not a Secret.")

    data = secret.get_data()
    secret_name = secret.get_name()

    env_secrets = []

    for key, _ in data.items():
        secret_env = {
            "name": key,
            "valueFrom": {
                "secretKeyRef": {
                    "name": secret_name,
                    "key": key
                }
            },
        }
        env_secrets.append(secret_env)

    return env_secrets


def _resources(resources):
    """ Generate the Argo YAML resource config for container to run.
    For example, the resource config in
    https://github.com/argoproj/argo/blob/master/examples/README.md#hello-world.
    """

    if isinstance(resources, dict):
        resource_ = {
            "requests": resources,
            # to fix the mojibake issue when dump yaml for one object
            "limits": copy.deepcopy(resources),
        }
        return resource_
    else:
        raise TypeError("container resource config need to be a dict")


def _create_job(manifest,
                action="create",
                success_condition=None,
                failure_condition=None):
    if manifest is None:
        raise ValueError("Input manifest can not be null")

    resource = OrderedDict()
    resource["action"] = action
    resource["setOwnerReference"] = "true"
    if success_condition:
        resource["successCondition"] = success_condition
    if failure_condition:
        resource["failureCondition"] = failure_condition
    resource["manifest"] = manifest
    return resource


def _update_pod_config(template):

    if _cluster_config is not None:
        template = _cluster_config.with_pod(template)

    return template


def _update_workflow_spec(spec):

    if _cluster_config is not None and \
            hasattr(_cluster_config, "with_workflow_spec"):
        spec = _cluster_config.with_workflow_spec(spec)

    return spec


def secret(secret_data, name, dry_run):
    return Secret(secret_data, name, dry_run)


def clean_workflow_after_seconds_finished(seconds):
    global _workflow_ttl_cleaned
    _workflow_ttl_cleaned = seconds


class Secret:
    def __init__(self, secret_data, name=None, dry_run=False):
        self.data = secret_data
        if name is not None:
            self.name = name
        else:
            self.name = "couler-" + str(uuid.uuid4())
        if not dry_run:
            global _secrets
            _secrets = self.generate_secret_yaml()

    def get_data(self):
        return self.data

    def get_name(self):
        return self.name

    def generate_secret_yaml(self):
        secret_yaml = {
            "apiVersion": "v1",
            "kind": "Secret",
            "metadata": {
                "name": self.name
            },
            "type": "Opaque",
        }

        secret_yaml["data"] = {}
        for key, value in self.data.items():
            encode_val = pyfunc.encode_base64(value)
            secret_yaml["data"][key] = encode_val

        return secret_yaml


# Dump the YAML when exiting
atexit.register(_dump_yaml)
