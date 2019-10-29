# Model Zoo on PAI TensorFlow

## Introduction

[Alibaba PAI](https://www.alibabacloud.com/product/machine-learning) is an end-to-end platform that provides various machine learning algorithms to meet users' data mining and analysis requirements. PAI supports TensorFlow and many other machine learning frameworks. SQLFlow model zoo also works on PAI. [Model Zoo Design Doc](./design_model_zoo.md) is a high level design for SQLFlow model zoo. This document is about how to do model training, model prediction, and model analysis using SQLFlow model zoo on PAI.

## Background
### Submit a Task to PAI
A PAI program is a normal program in TensorFlow or other framework, except that PAI requires its application expose several command line options to communicate with [MaxCompute](https://www.alibabacloud.com/product/maxcompute) and OSS([Alibaba Cloud Object Storage Service](https://www.alibabacloud.com/product/oss)). Typically, a user submit a PAI task with one of the following methods:
1. Use the MaxCompute CLI `odpscmd`:
```bash
$ odpscmd -e "pai -name tensorflow -Dscript=file:///my/training/script.py -Dother_param=other_param_value..."

```
When a user runs the command above, the CLI will submit the task to PAI then print a logview URL and an instance ID to stdout, after which the user can safely terminate the CLI. The user can check the logview to get the running status of the task or run:
```bash
$ odpscmd -e "wait instanceID"
```
to attach to the running task.


2. Use the MaxCompute Python Package `pyodps`:
```python

from odps import ODPS

conn = ODPS(...)
inst = conn.run_xflow('tensorflow', parameters={'script':      'file:///my/training/script.py',
               	                                'other_param': 'other_param_value' ... })
print(instance.get_logview_address())
```
This code snippet asynchronously submits a task to PAI and prints the logview URL to stdout.

### Train/Predict/Analyze in SQLFlow with Model Zoo
In SQLFlow, we train a model with the following SQLFlow extened SELECT statement:
```sql
SELECT * FROM train_table ... TO TRAIN MyAwesomeEstimator ... INTO my_awesome_model;
```
and predict with:
```sql
SELECT * FROM pred_table ... TO PREDICT using my_awesome_model ... ;
```
and analyze with:
```sql
SELECT * FROM train_table ...  ANALYZE my_awesome_model USING TreeExplainer;

```
The extended SELECT statements are executed as described in [Model Zoo Design Doc](./design_model_zoo.md#Submitter-Programs):
- If the model **is not** from a model zoo, The SQLFlow server converts these statements to a *submitter* file and submits the file to a specified engine.
- If the model **is** from a model zoo, the submmiter program is mapped into a Docker image. the SQLFlow server calls k8s API to launch the Docker image on a k8s cluster.

## The Design
### Base Image
The base image of the SQLFlow model zoo should incorporate both odpscmd(which is already in place) and pyodps.  We propose to use `odpscmd` at the moment because it requires less code modification in both python and golang.

### Submit a SQLFlow Task to PAI With Model Zoo
When a user submits a SELECT statement as above, SQLFlow should take the following actions:
1. SQLFlow Checks whether the entity after `TO TRAIN/PREDICT/ANALYZE` is from a SQLFlow model zoo. For example, a plain `DNNClassier` implies that the model is a premade estimator, and `"models.sqlflow.org/sqlflow/my_awesome_model"` implies that the model is from model zoo "models.sqlflow.org". The actual mechanism may be more complicated and is still under progress.
1. Case A, the model **is not** from a model zoo:
    1. Case A.1: the user specifies that the task should be submitted to PAI (For example, the user set a environment variable such as `SQLFLOW_submitter=PAI`).
        - The SQLFlow server generates a submitter script with PAI-required command line options.
		- The SQLFlow server uses odpscmd or pyodps to submit the script to PAI as [described above](#Submit-a-Task-to-PAI). 
    1. Case A.2: the task is not to be submitted to PAI:
	    - Run the other way.
1. Case B: the model **is** from a model zoo:
    1. Case B.1: the user specifies that the task should be submitted to PAI.
    1. Case B.2: the user doesn't specify where to submit but the model knows where. For example, a model zoo table field may specify the platform to submit to.
	    - The SQLFlow server generates a submitter script with PAI-required command line options.
        - SQLFlow pulls the Docker image and calls k8s API to launch it on a k8s cluster to execute the following command:
        ```bash
        docker run --rm -it \
          -v /var/sqlflow/submitters:/submitters sqlflow/my_awesome_model \
            odpscmd -Dscript=/submitters/sqlflow/my_awesome_model.py ...
        ```
    1. Case B.3: the task is not to be submitted to PAI.
	    - Run the other way.

### PAI Model Zoo Data Schema
For security reasons, we propose to leverage the existing user access control of MaxCompute. As a result of this consideration, the PAI model zoo table should be built on MaxCompute, which is typically the data source of a PAI training program. The model zoo table of PAI should contain the following fields:

| model ID | Docker image ID | submitter program | data converter | model parameter file path | metrics | datetime | logview | statement | SQLFlow version | platform | name |
|----------|-----------------|-------------------|----------------|---------------------------|---------|----------|---------|-----------|-----------------|----------|------|
|          |                 |                   |                |                           |         |          |         |           |                 |          |      |

1. *model ID*, *Docker image ID*, *submitter program*, *data converter*, *model parameter file path*, these fields is described in [Model Zoo Design Doc](./design_model_zoo.md)
1. *metrics*, the metrics that measure the training results, e.g. AUC, loss, F1 etc.
1. *datetime*, a timestamp when the user start training.
1. *logview*, logview URL of a PAI task.
1. *statement*, the SQL statement which submitted the training task.
1. *SQLFlow version*, the version of SQLFlow which generated the submitter program.
1. *platform*, defaults to "PAI"
1. *name*, the same meaning as its namesake in `odpscmd -e "pai -name ...`, defaults to "tensorflow"

The last seven fields is used to ease usage on PAI. 

### Model Sharing and Publication
For security reasons, in addition to models.sqlflow.org, we propose to deploy a private Docker registry with stricter access control for model publication and model sharing. Each user can enjoy all the models authorized from both public and private repositories.
