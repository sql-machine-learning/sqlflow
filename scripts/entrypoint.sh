#!/bin/bash

# Enable Python virtual environment for non-interactive bash
. /miniconda/etc/profile.d/conda.sh
source activate sqlflow-dev

exec $@
