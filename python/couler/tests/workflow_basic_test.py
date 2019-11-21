# Copyright 2019 The SQLFlow Authors. All rights reserved.
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
from collections import OrderedDict

import couler.argo as couler


def random_code():
    import random

    result = "heads" if random.randint(0, 1) == 0 else "tails"
    print(result)


def flip_coin():
    return couler.run_script(image="python:alpine3.6", source=random_code)


def heads():
    return couler.run_container(
        image="alpine:3.6", command='echo "it was heads"'
    )


def tails():
    return couler.run_container(
        image="alpine:3.6", command='echo "it was tails"'
    )


class WorkflowBasicTest(unittest.TestCase):
    def setUp(self):
        couler._cleanup()

    def test_steps_order(self):
        flip_coin()
        heads()
        tails()
        flip_coin()

        steps = list(couler._steps.values())
        hope_steps = [
            [{"name": "flip-coin-35", "template": "flip-coin"}],
            [{"name": "heads-36", "template": "heads"}],
            [{"name": "tails-37", "template": "tails"}],
            [{"name": "flip-coin-38", "template": "flip-coin"}],
        ]

        self.assertEqual(steps, hope_steps)

    def test_when_order_two(self):
        couler.steps = OrderedDict()
        couler.update_steps = True
        couler.when(couler.equal(flip_coin(), "heads"), lambda: heads())
        couler.when(couler.equal(flip_coin(), "tails"), lambda: tails())

        steps = list(couler._steps.values())
        hope_steps = [
            [{"name": "flip-coin-53", "template": "flip-coin"}],
            [
                {
                    "name": "heads-53",
                    "template": "heads",
                    "when": "{{steps.flip-coin-53.outputs.result}} == heads",
                }
            ],
            [{"name": "flip-coin-54", "template": "flip-coin"}],
            [
                {
                    "name": "tails-54",
                    "template": "tails",
                    "when": "{{steps.flip-coin-54.outputs.result}} == tails",
                }
            ],
        ]

        self.assertEqual(hope_steps, steps)

    def test_when_order_three(self):
        couler.steps = OrderedDict()
        couler.update_steps = True
        output_1 = flip_coin()
        couler.when(couler.equal(output_1, "heads"), lambda: heads())

        output_2 = flip_coin()
        couler.when(couler.equal(output_1, "tails"), lambda: tails())
        couler.when(couler.equal(output_2, "heads"), lambda: heads())
        couler.when(couler.equal(output_2, "tails"), lambda: tails())

        steps = list(couler._steps.values())
        hope_steps = [
            [{"name": "flip-coin-81", "template": "flip-coin"}],
            [
                {
                    "name": "heads-82",
                    "template": "heads",
                    "when": "{{steps.flip-coin-81.outputs.result}} == heads",
                },
                {
                    "name": "tails-85",
                    "template": "tails",
                    "when": "{{steps.flip-coin-81.outputs.result}} == tails",
                },
            ],
            [{"name": "flip-coin-84", "template": "flip-coin"}],
            [
                {
                    "name": "heads-86",
                    "template": "heads",
                    "when": "{{steps.flip-coin-84.outputs.result}} == heads",
                },
                {
                    "name": "tails-87",
                    "template": "tails",
                    "when": "{{steps.flip-coin-84.outputs.result}} == tails",
                },
            ],
        ]
        self.assertEqual(hope_steps, steps)

    def test_when_with_parameter(self):
        def output_message(message):
            return couler.run_container(
                image="docker/whalesay:latest",
                command=["cowsay"],
                args=[message],
            )

        number = flip_coin()
        couler.when(
            couler.bigger(number, "0.2"), lambda: output_message(number)
        )

        steps = list(couler._steps.values())
        expected = [
            [
                OrderedDict(
                    [("name", "flip-coin-128"), ("template", "flip-coin")]
                )
            ],
            [
                OrderedDict(
                    [
                        ("name", "output-message-130"),
                        ("template", "output-message"),
                        (
                            "when",
                            "{{steps.flip-coin-128.outputs.result}} > 0.2",
                        ),
                        (
                            "arguments",
                            {
                                "parameters": [
                                    {
                                        "name": "para-output-message-0",
                                        "value": '"{{steps.flip-coin-128.outputs.result}}"',  # noqa: E501
                                    }
                                ]
                            },
                        ),
                    ]
                )
            ],
        ]
        self.assertEqual(steps, expected)
