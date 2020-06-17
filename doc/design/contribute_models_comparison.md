# Comparisons of How to Contribute Models Designs

## A General View of the Designs

I put currently available designs of model contributor local tool requirements in the below table for comparison of the designs.


| tool chain      | detail steps |
| --------------- | ------------ |
| Docker          | [contribute_models.md](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/contribute_models.md) |
| VM (VirtualBox) | [sqlflow playground](https://github.com/sql-machine-learning/playground) |
| Embedded VM     | just like [minikube](https://kubernetes.io/docs/setup/learning-environment/minikube/) |
| Python API      | [contribute_models_new.md](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/contribute_models_new.md) |
| Native Model    | write and test models using native Keras, Pytorch |


## Principles When Choosing the Design

We follow below two major principles when choosing the design:

1. The requirements of each user role should be satisfied.
1. Do not keep redundant branches of code, like local mode and workflow mode; VM deployment and Docker deployment.

## Design Decisions

Here we make decisions on these choices:

### Use VM playground or Docker image?

**Use VM for SQLFlow development environment and playground deployments.**

- We should keep only workflow mode according to **principle two**.
- For SQLFlow contributors, it's natual to have a local developing and debugging environment which a VM should be simple to use because setting up minikube and deploy MySQL, sqlflowserver in the minikube takes a lot of time. Use the pre-built VM box simplifies this job.
- For normal users, the just need a place to write SQLFlow statements and execute them. For on-premise environments, there will be web GUI in any form. For public access, we can provide a cloud playground so that normal users won't bother setting up an local VM.
- For model developers, they just need a place to test the model. Since the model should be built into Docker images before SQLFlow can generate a workflow step that uses the model, using a local VM can not reduce the debugging time than using a central sqlflow server deployment. And more, model contributors may not familar with setting VM locally, which may takes a lot of work for him. We should let the model contributors cares only about the model's code. It will be cool if a model contributor can just write native model code, test it locally using native code, then use `sqlflow release repo my_model_dir/` to the on-premise model zoo server or a public model zoo server, then test it out on the web GUI or our public playground. For comparision, below are steps to test a model using a local VM and a cloud playground:
    - local VM:
        1. setup local VM
        1. either use `sqlflow release repo my_model/` or build the model Docker image inside the VM.
        1. execute a statement using the model image
    - cloud playground (on-premise web GUI)
        1. use `sqlflow release repo my_model/` to release the model.
        1. execute a statement using the model image
    

### Do we provide cloud playground or let users setup the playground VM box on their laptop?

**Both**.

The VM box deployed locally can also be deployed on the cloud for multiple users. This follows the **principle two**

For SQLFlow developers, we can use the VM box to set up a local debugging environment.
For normal users and model contributors, the should use the cloud playground to do some test.

### Do we need SQLFlow Python API?

**No**.

If we can let model contributors write any native model code (any Keras model, Pytorch model etc.), and SQLFlow do not set constraints on how users should write the model, contributing models then will become simple and straight forward. Any model developed by the contributor can be adapted to SQLFlow, and the contributor event don't care whether the model will work with SQLFlow, he just write it and test it in the native way.


## Conclusion

We prefer the "native model" method for model contributors to develop and test SQLFlow models, to give the best experience to model contributors and all other roles.

We maintains an playground VM box for SQLFlow developers to ease the debugging environment setup.

We maintains a cloud playground deployments publicly for public users to contribute and test their models.

For each on-premise deployments, normal users and model contributors can test their model on the on-premise web GUI inside the company.

## Appendix

### What is a "Native Model"

Currently, if we want a custom Keras model can work with SQLFlow, we should have required init function arguments and other function definitions in the model definition Python file, like: https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/dnnclassifier.py:

1. The model class `__init__` function must have the argument `feature_columns` and process this argument to generate input layers in the `__init__` function.
1. In the Python file, there must be functions `optimizer`, `loss`, `prepare_prediction_column`, and `eval_metrics_fn` for SQLFlow to compile the model properly.

A "native model" means we remove above all restrictions, any Keras model definition can work with SQLFlow.