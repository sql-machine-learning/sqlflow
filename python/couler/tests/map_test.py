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

import couler.argo as couler


def consume(message):
    return couler.run_container(
        image="docker/whalesay:latest", command=["cowsay"], args=[message]
    )


class MapTest(unittest.TestCase):
    def test_map_function(self):
        test_paras = ["t1", "t2", "t3"]
        couler.map(lambda x: consume(x), test_paras)
        wf = couler.yaml()
        templates = wf["spec"]["templates"]
        self.assertEqual(len(templates), 2)

        # We should have a 'consume' template
        consume_template = templates[1]
        self.assertEqual(consume_template["name"], "consume")
        # Check input parameters
        expected_paras = [{"name": "para-consume-0"}]
        self.assertListEqual(
            consume_template["inputs"]["parameters"], expected_paras
        )
        # Check container
        expected_container = {
            "image": "docker/whalesay:latest",
            "command": ["bash", "-c", "cowsay"],
            "args": ['"{{inputs.parameters.para-consume-0}}"'],
        }
        self.assertDictEqual(consume_template["container"], expected_container)

        # Check the steps template
        steps_template = templates[0]
        self.assertEqual(steps_template["name"], "pytest")
        self.assertEqual(len(steps_template["steps"]), 1)
        self.assertEqual(len(steps_template["steps"][0]), 1)
        map_step = steps_template["steps"][0][0]
        self.assertEqual(map_step["template"], "consume")
        # Check arguments
        expected_paras = [
            {"name": "para-consume-0", "value": '"{{item.para-consume-0}}"'}
        ]
        self.assertListEqual(
            map_step["arguments"]["parameters"], expected_paras
        )
        # Check withItems
        expected_with_items = [
            {"para-consume-0": "t1"},
            {"para-consume-0": "t2"},
            {"para-consume-0": "t3"},
        ]
        self.assertListEqual(map_step["withItems"], expected_with_items)
