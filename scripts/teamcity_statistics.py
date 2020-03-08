#!/bin/env python
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
import re
import xml.etree.ElementTree as ET


class TeamcityReport(object):
    """
    TeamcityReport reports a custom statistics on TeamCity system.
    Ref: https://www.jetbrains.com/help/teamcity/build-script-interaction-with-teamcity.html#BuildScriptInteractionwithTeamCity-ReportingCustomStatistics 
    """
    def __init__(self, out):
        self._out = out
        self._metrics = []

    def add_metrics(self, key, value):
        self._metrics.append((key, value))

    def flush(self):
        root = ET.Element('build')
        for m in self._metrics:
            ET.SubElement(root,
                          "statisticValue",
                          attrib={
                              "key": str(m[0]),
                              "value": str(m[1])
                          })
        tree = ET.ElementTree(root)
        tree.write(self._out)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Process some integers.')
    parser.add_argument('--file',
                        type=str,
                        required=True,
                        help='a file of `gotest -v | tee ...` ')
    parser.add_argument(
        '--case',
        type=str,
        required=True,
        help='matching the regular expression test cases of Go')
    args = parser.parse_args()
    # the XML file name should be `teamcity-info.xml`
    r = TeamcityReport("teamcity-info.xml")
    with open(args.file) as f:
        for line in f:
            content = line.strip()
            if len(content) == 0:
                continue
            m = re.search('PASS: (%s.*) \((.*)s\)' % args.case, content)
            if m:
                r.add_metrics(m.group(1), m.group(2))
    r.flush()
