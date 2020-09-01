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

import sys

import matplotlib
# The default backend
import matplotlib.pyplot as plt
from runtime.model.oss import copyfileobj

# TODO(shendiaomo): extract common code from tensorflow/explain.py
# and xgboost/explain.py
# TODO(shendiaomo): add a unit test for this file later


def plot_and_save(plotfunc,
                  oss_dest=None,
                  oss_ak=None,
                  oss_sk=None,
                  oss_endpoint=None,
                  oss_bucket_name=None,
                  filename='summary'):
    '''
    plot_and_save plots and saves matplotlib figures using different backends
    Args:
        plotfunc: A callable that plot the figures
        oss_dest: The oss path to save the figures
        oss_ak: The access key of the oss service
        oss_sk: The security key of the oss service
        oss_endpoint: The endpoint of the oss service
        oss_bucket_name: The bucket name of the oss service
        filename: The prefix of the figure files to be saved
    Return:
        None
    '''

    plotfunc()
    plt.savefig(filename, bbox_inches='tight')

    if oss_dest:
        copyfileobj(filename + '.png', oss_dest, oss_ak, oss_sk, oss_endpoint,
                    oss_bucket_name)
    else:
        # NOTE(weiguoz), I failed test on the PAI platform here.
        # If we plan to support plotille_text_backend on PAI, please test it.
        # The plotille text backend
        matplotlib.use('module://plotille_text_backend')
        import matplotlib.pyplot as plt_text_backend
        sys.stdout = PseudoTTY(sys.stdout)
        plotfunc()
        plt_text_backend.savefig(filename, bbox_inches='tight')


class PseudoTTY(object):
    def __init__(self, underlying):
        self.__underlying = underlying

    def __getattr__(self, name):
        return getattr(self.__underlying, name)

    def isatty(self):
        return True
