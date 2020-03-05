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

import argparse
import os
from subprocess import call


def add_run_params(parser):
    parser.add_argument("--mode",
                        type=str,
                        help="python or argo",
                        required=True)
    # TODO: can skip --file, directly run as "couler run file1.py"
    parser.add_argument("--file", type=str, required=True)
    parser.add_argument("--workflow_name", type=str, default='sqlflow')
    parser.add_argument(
        "--cluster_config",
        default=None,
        type=str,
        help="Path of the cluster specific configuration file",
        required=False,
    )


def run(args):
    if args.cluster_config is not None:
        cluster_config_path = args.cluster_config
        os.environ["couler_cluster_config"] = cluster_config_path

    os.environ["workflow_name"] = args.workflow_name

    # TODO(yancey1989):remove subprocess and the `mode` argument
    return call(["python", args.file], env=os.environ)


def main():
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="run")
    subparsers.required = True

    run_parser = subparsers.add_parser("run", help="to be added")
    run_parser.set_defaults(func=run)
    add_run_params(run_parser)

    args, _ = parser.parse_known_args()
    return args.func(args)


if __name__ == "__main__":
    main()
