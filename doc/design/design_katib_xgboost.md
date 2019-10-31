# Katib XGboost on SQLFlow

## Overview:
This is a doc on integration with katib XGboost.

## User Interface:


## Implementation:

Steps:
1. Based on input SQL statement, the codegen `codegen_katib_xgboost.go` generates `katib_xgboost.py` file. This python program:
   1. generates `train_xgboost.py` file. All input parameters will be filled in `train_xgboost.py`.
   2. gets all input data files (e.g., `train_data.txt`).
   3. generates a Dockerfile and build a xgboost docker image from this Dockerfile. The `train_xgboost.py` and required data file will be copied into this image.
   4. pushes this docker image into docker.io repository.
   5. generates `katib_xgboost.yaml` file and fill it with: (1) source of docker image generated above; and (2) commands to execute `train_xgboost.py` in the container.
   5. to submit and execute `katib_xgboost.yaml` on kubernetes.
3. Katib creates an experiments to run XGboost job:
   1. Kubernetes create a Suggestion which generates value of hyperparameters;
   2. Kubernetes create multiple Trial Pods to execute xgboost traning job. Each Trial job includes two containers: (1) one is executing xgboost job; (2) a MetricsCollector collect and parse logs from the other container.
   3. The MetricsCollector write data into the store of Katib.

