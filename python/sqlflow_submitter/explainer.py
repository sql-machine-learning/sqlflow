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

import sys

import matplotlib
import matplotlib.pyplot as plt

# TODO(shendiaomo): extract common code from tensorflow/explain.py and xgboost/explain.py
# TODO(shendiaomo): add a unit test for this file later

def plot_and_save(plotfunc, filename='summary'):
    '''
    plot_and_save plots and saves matplotlib figures using different backends
    Args:
        plotfunc: A callable that plot the figures
        filename: The prefix of the figure files to be saved
    Return:
        None
    '''

    # The default backend 
    plotfunc()
    plt.savefig(filename, bbox_inches='tight')

    # The plotille text backend 
    matplotlib.use('module://plotille_text_backend')
    import matplotlib.pyplot as plt_text_backend
    sys.stdout.isatty = lambda:True
    plotfunc()
    plt_text_backend.savefig(filename, bbox_inches='tight')

