# Model Zoo on Alibaba PAI

## Introduction

[Alibaba PAI](https://www.alibabacloud.com/product/machine-learning) is an end-to-end platform that provides various machine learning algorithms to meet users' data mining and analysis requirements. PAI supports TensorFlow and many other machine learning frameworks. SQLFlow model zoo also works on PAI. [Model Zoo Design Doc](model_zoo.md) is a high level design for SQLFlow model zoo. This document is about how to do model training, model prediction, and model analysis using SQLFlow model zoo on PAI.

## Concepts

Besides what described in [Model Zoo Concepts](model_zoo.md#Concepts), there are several new concepts about Alibaba PAI.

1. The **PAI platform** or **PAI** for short is an end-to-end platform that provides various machine learning algorithms to meet your data mining and analysis requirements. PAI supports TensorFlow and many other machine learning frameworks. See [PAI Introduction](https://www.alibabacloud.com/help/doc-detail/67461.htm) for more details.

1. **PAI TensorFlow** is the deployment of TensorFlow on **PAI**, with necessary optimization and further development that makes it possible to cooperate efficiently with other [Alibaba Cloud](https://www.alibabacloud.com/) components such as MaxCompute.

1. A **PAI program** is a python program that is developed base on TensorFlow, MXNet or [other machine learning frameworks supported by PAI](https://www.alibabacloud.com/help/doc-detail/69688.htm).

1. A **PAI task** is an instance of a **PAI program** that is being executed by the **PAI platform**.

## Background

The background of SQLFlow has been discussed thoroughly in [Model Zoo Background](model_zoo.md#Background). We'll explain how to submit a *PAI task* to the *PAI platform* in this section.

PAI requires a *PAI program* to expose several command line options to communicate with [MaxCompute](https://www.alibabacloud.com/product/maxcompute) and OSS([Alibaba Cloud Object Storage Service](https://www.alibabacloud.com/product/oss)). Typically, a user can submit the PAI program by:

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

## The Design

### Versioning and Releasing

Versioning and releasing in PAI model zoo is the same as what's described in [Model Zoo Design](model_zoo.md#Versioning-and-Releasing). The only requirement is that the base docker image of the SQLFlow model zoo should incorporate both `odpscmd`(which is already in place) and `pyodps`. We propose to use `pyodps` at the moment because it is easier to be integrated into the new workflow framework that is in progress.

### Submitter Programs of PAI Model Zoo

We propose to require the model source directory of a model zoo Docker image to be a legal python package with the same name as the image itself, besides, the python package should expose all the model classes. In that case, whether or not a model is executed on PAI, its [submitter program](model_zoo.md#Submitter-Programs) can easily import the *model definition*s.

For example, if we refer to a *model definition* `MyAwesomeModel` by `sqlflow/my_awesome_model/MyAwesomeModel`, it implies that:
- the Docker image `sqlflow/my_awesome_model` has a directory `/my_awesome_model`
- the *model definition* is a python class `MyAwesomeModel` defined in a python script `/my_awesome_model/name_as_you_wish.py`
- the file `/my_awesome_model/__init__.py` has a line `from .name_as_you_wish import DNNClassifier`

Considering the above example, SQLFlow should:

1. For a model zoo model that is to be executed in a Docker container, SQLFlow generates **one** submitter program:
    - `submitter.py` would run in a Docker container as described in [Submitter Programs](model_zoo.md#Submitter-Programs)
    ```python
	import os
	import sys
	sys.path += ['/']
	import my_awesome_model

	# Do my stuff here ...
	```

1. For a model zoo model that is to be executed on PAI, SQLFlow generates **two** programs, i.e. a submitter program and a PAI entry file:
    - `pai_entry.py` would run on PAI as described in [Background](#Background)
    ```python
	import my_awesome_model
	# Define all the PAI required command line options
	# Do my stuff here ...
	```

    - `submitter.py` would run in a Docker container as described in [Submitter Programs](model_zoo.md#Submitter-Programs)
    ```python
	import os
    import tarfile
	from odps import ODPS
    
	TARBALL = "/submitters/my_awesome_model.tar.gz"
    archive = tarfile.open(tarball, "w|gz")

    # '.' is always in sys.path
    archive.add('/my_awesome_model')
    archive.add('/submitters/pai_entry.py', arcname=".")
    archive.add('/submitters/requirements.txt', arcname=".")  # ** see below
    archive.close()

	# Submit the tarball to PAI
    conn = ODPS(...)
    inst = conn.run_xflow("tensorflow",
                          "my_project",
                          parameters={"script": "file://" + TARBALL,
                                      "entry":  "pai_entry.py",
                                      "other_param": "other_param_value" ... })
	```

    ** Considering that `MyAwesomeModel` may depend on third party python packages that are not in place on PAI, we propose to introduce a mechanism to analyze the model's dependencies automatically before submitting it to PAI. The mechanism may be simply implemented in the Docker container as a `pip list` command followed by a set subtraction command(e.g. [the comm utility](https://linux.die.net/man/1/comm)), the result file is packed into the tarball that is to be submitted to PAI.

Now let's sum up. Currently, each deployment of SQLFlow has been configured to use only one platform. So we assume that all the tasks of the deployment of SQLFlow on PAI will be submitted to PAI.
When a user submits a SELECT statement as described in [Model Zoo Background](model_zoo.md#Background), SQLFlow should take the following actions:

1. SQLFlow Checks whether the entity after `TO TRAIN/PREDICT/EXPLAIN` is from a SQLFlow model zoo. For example, `"models.sqlflow.org/sqlflow/my_awesome_model/MyAwesomeModel"` implies that the model is from model zoo `models.sqlflow.org`, and a  plain `DNNClassier` implies that the model is a premade estimator. The actual mechanism may be more complicated and is still under progress.

1. Case A: the model **is not** from a model zoo:
    - The SQLFlow server generates a submitter program and a single-file PAI program.
	- There are no model source files. SQLFlow calls the submitter program to submit the PAI program to PAI.

1. Case B: the model **is** from a model zoo:
    - The SQLFlow server generates a submitter program and a single-file PAI program.
    - SQLFlow pulls the Docker image and calls k8s API to launch it on a k8s cluster to execute the following command:
        ```bash
        docker run --rm -it \
          -v /var/sqlflow/submitters:/submitters sqlflow/my_awesome_model \
             python /submitters/submitter.py
        ```
    - There are model source files. The submitter program does the following in the Docker container as the [code snippet above](#L85-L106):
        - Analyzes model dependencies.
        - Packs all the dependencies as well as the model source files to a tarball.
        - Submits the tarball to PAI.

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

### Model Sharing in PAI Model Zoo

Model sharing in PAI model zoo is nearly the same as [Model Zoo Model Sharing](model_zoo.md#Model-Sharing). The only difference is, for security reasons, users can only access the models they are authorized.
For example, suppose there is a model `my_first_model` that is trained by `an_analyst`, if another analyst wants to use the trained model, she not only needs to use the full name `an_analyst/my_first_model`, but also needs to have access to the model. The access control mechanism is based on an SSO system or similar systems.

### Model Publication in PAI Model Zoo

For the same security reasons, in addition to `models.sqlflow.org`, we propose to deploy a private Docker registry with stricter access control for model publication. Each user can enjoy all the models authorized from both public and private repositories. Other than this, model publication in PAI model zoo is the same as [Model Zoo Model Publication](model_zoo.md#Model-Publication). 

We encourage model developers to develop their models in high level TensorFlow APIs(tf.keras) to ensure TensorFlow version compatibility.
