#!/bin/bash

echo "Setup Jupyter notebook connecting to $SQLFLOW_SERVER ..."
jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''
