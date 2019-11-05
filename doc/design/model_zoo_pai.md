# Model Zoo on Alibaba PAI

## Introduction

[Alibaba PAI](https://www.alibabacloud.com/product/machine-learning) is an end-to-end platform that provides various machine learning algorithms to meet users' data mining and analysis requirements. PAI supports TensorFlow and many other machine learning frameworks. SQLFlow model zoo also works on PAI. [Model Zoo Design Doc](model_zoo.md) is a high level design for SQLFlow model zoo. This document is about how to do model training, model prediction, and model analysis using SQLFlow model zoo on PAI.

## Background

### Submit a Task to PAI

A PAI program is a python program that uses TensorFlow or other frameworks like PyTorch. PAI requires the python program to expose several command line options to communicate with [MaxCompute](https://www.alibabacloud.com/product/maxcompute) and OSS([Alibaba Cloud Object Storage Service](https://www.alibabacloud.com/product/oss)). Typically, a user can submit the PAI program by:

1. Use the MaxCompute CLI program [`odpscmd`](https://www.alibabacloud.com/help/doc-detail/27971.htm):

    ```bash
    $ odpscmd -e "pai -name tensorflow -Dscript=file:///my/training/script.py -Dother_param=other_param_value..."
    
    ```

    When a user runs the above command, the CLI will submit the task to PAI then print a logview URL and an instance ID to stdout, after which the user can safely terminate the CLI. The user can check the logview to get the running status of the task or run:
    
    ```bash
    $ odpscmd -e "wait instanceID"
    ```
    
    to attach to the running task.

1. Use the MaxCompute Python Package [`pyodps`](https://pyodps.readthedocs.io/en/latest/):

    ```python
    from odps import ODPS
    
    conn = ODPS(...)
    inst = conn.run_xflow('tensorflow', parameters={'script':      'file:///my/training/script.py',
                   	                                'other_param': 'other_param_value' ... })
    print(instance.get_logview_address())
    ```
    
    This code snippet asynchronously submits a task to PAI and prints the logview URL to stdout.

**Note** that the code snippets here are only for demonstration. In practice, there may be multiple python scripts in a PAI program. If so, we can pack all the python scripts/dependencies of the PAI program into a tarball that can be passed to the `-Dscript` option. PAI will automatically unpack the tarball. And PAI has another command line option to specify which python script is the entry of the program. For details, please refer to the user guide of [odpscmd](https://www.alibabacloud.com/help/doc-detail/27971.htm) and [pyodps](https://pyodps.readthedocs.io/en/latest/).

### Train/Predict/Analyze in SQLFlow with Model Zoo

In SQLFlow, we train, predict or analyze a model with the following SQLFlow extended SELECT statement:

```sql
-- Train
SELECT * FROM train_table ... TO TRAIN MyAwesomeEstimator ... INTO my_awesome_model;
```

```sql
-- Predict
SELECT * FROM pred_table ... TO PREDICT using my_awesome_model ... ;
```

```sql
-- Analyze
SELECT * FROM train_table ...  TO ANALYZE my_awesome_model USING TreeExplainer;
```

The extended SELECT statements are executed as described in [Model Zoo Design Doc](model_zoo.md#Submitter-Programs):
- If the model **is not** from a model zoo, The SQLFlow server converts these statements to a *submitter* program and submits the program to a specified engine.
- If the model **is** from a model zoo, the submmiter program is mounted into a Docker image. The SQLFlow server calls k8s API to launch the Docker image on a k8s cluster.

## Concepts

Besides what described in [Model Zoo Concepts](model_zoo.md#Concepts), there are several new concepts about Alibaba PAI.

1. The **PAI platform** or **PAI** for short is an end-to-end platform that provides various machine learning algorithms to meet your data mining and analysis requirements. PAI supports TensorFlow and many other machine learning frameworks. See [PAI Introduction](https://www.alibabacloud.com/help/doc-detail/67461.htm) for more details.

1. **PAI TensorFlow** is the deployment of TensorFlow on **PAI**, with necessary optimization and further development that makes it possible to cooperate efficiently with other [Alibaba Cloud](https://www.alibabacloud.com/) components such as MaxCompute.

1. A **PAI program** is a python program that is developed base on TensorFlow, MXNet or [other machine learning frameworks supported by PAI](https://www.alibabacloud.com/help/doc-detail/69688.htm).

1. A **PAI task** is an instance of a **PAI program** that is being executed by the **PAI platform**.

## The Design

### Versioning and Releasing

Versioning and releasing in PAI model zoo is the same as what's described in [Model Zoo Design](model_zoo.md#Versioning-and-Releasing). The only requirement is that the base docker image of the SQLFlow model zoo should incorporate both `odpscmd`(which is already in place) and `pyodps`.  We propose to use `odpscmd` at the moment because it requires less code modification in both python and golang.

### Submitter Programs of PAI Model Zoo

Currently, each deployment of SQLFlow has been configured to use only one submitter. So we assume that all the tasks of the deployment of SQLFlow on PAI will be submitted to PAI.

When a user submits a SELECT statement as above, SQLFlow should take the following actions:

1. SQLFlow Checks whether the entity after `TO TRAIN/PREDICT/ANALYZE` is from a SQLFlow model zoo. For example, a plain `DNNClassier` implies that the model is a premade estimator, and `"models.sqlflow.org/sqlflow/my_awesome_model"` implies that the model is from model zoo "models.sqlflow.org". The actual mechanism may be more complicated and is still under progress.
1. Case A, the model **is not** from a model zoo:
    - The SQLFlow server generates a submitter program with PAI-required command line options.
	- The SQLFlow server uses `odpscmd` or `pyodps` to submit the program to PAI as [described above](#Submit-a-Task-to-PAI). 
1. Case B: the model **is** from a model zoo:
    - The SQLFlow server generates a submitter program from the model-zoo model with PAI-required command line options.
    - SQLFlow pulls the Docker image and calls k8s API to launch it on a k8s cluster to execute the following command:

        ```bash
        docker run --rm -it \
          -v /var/sqlflow/submitters:/submitters sqlflow/my_awesome_model \
            odpscmd  -e 'pai -name tensorflow -Dscript=/submitters/sqlflow/my_awesome_model.tar.gz ...'
        ```

### PAI Model Zoo Data Schema

For security reasons, we propose to leverage the existing user access control of MaxCompute. As a result of this consideration, the PAI model zoo table should be built on MaxCompute, which is typically the data source of a PAI training program. The model zoo table of PAI should contain the following fields:

| model ID | creator | model zoo release | model definition | submitter program | data converter | model parameter file path | metrics | datetime | logview | statement | SQLFlow version | name |
|----------|---------|-------------------|------------------|-------------------|----------------|---------------------------|---------|----------|---------|-----------|-----------------|------|

1. *model ID*
1. *creator*
1. *model zoo release*
1. *model definition*
1. *submitter program*
1. *data converter*
1. *model parameter file path*
1. *metrics*, the metrics that measure the training results, e.g. AUC, loss, F1 etc.
1. *datetime*, a timestamp when the user start training.
1. *logview*, logview URL of a PAI task.
1. *statement*, the SQL statement which submitted the training task.
1. *SQLFlow version*, the version of SQLFlow which generated the submitter program.
1. *name*, the same meaning as its namesake in `odpscmd -e "pai -name ...`, defaults to "tensorflow"

The 1st to 7th fields are consistent with the [Model Zoo Data Schema](model_zoo.md#Model-Zoo-Data-Schema) where they are introduced. The 8th to 13th fields are used to ease usage on PAI, because they are only useful for users of PAI model zoo, we can simply omit these extra fields when publishing a PAI model to `model.sqlflow.org`. Similarly, we can keep the extra fields unfilled when submitting a PAI task using a published model from `model.sqlflow.org`.

### Model Sharing

Model sharing in PAI model zoo is nearly the same as [Model Zoo Model Sharing](model_zoo.md#Model-Sharing). The only difference is, for security reasons, users can only access the models they are authorized.
For example, suppose there is a model `my_first_model` that is trained by `an_analyst`, if another analyst wants to use the trained model, she not only need to use the full name `an_analyst/my_first_model`, but also need to have access to the model. The access control mechanism is based on an SSO system or similar systems.

### Model Publication

For the same security reasons, in addition to models.sqlflow.org, we propose to deploy a private Docker registry with stricter access control for model publication. Each user can enjoy all the models authorized from both public and private repositories. Beyond that, model publication in PAI model zoo is the same as [Model Zoo Model Publication](model_zoo.md#Model-Publication). 
