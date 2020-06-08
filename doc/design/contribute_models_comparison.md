# Comparisons of How to Contribute Models

There are several roles of SQLFlow users, they are:

1. Users, they write SQLFlow statements to leverage the power of AI in real-world cases.
1. Model contributors ( or model developers ), they write model definitions and share the model definitions to users, typically they write Python code. Model contributors also write SQLFlow statements to test and use the model they have developed.
1. SQLFlow contributors, they develop, deploy, and maintain the deployments of SQLFlow.

We'd like to provide unified, easy to learn steps for all these roles to work with SQLFlow. In this document, we mainly discuss the designs for how the model contributors contribute models, to see which design is the best we can have for now.


## A General View of the Designs

I put currently available designs of model contributor local tool requirements in the below table for comparison of the designs.


| tool chain      | detail steps |
| --------------- | ------------ |
| Docker          | [contribute_models.md](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/contribute_models.md) |
| VM (VirtualBox) | [sqlflow playground](https://github.com/sql-machine-learning/playground) |
| Embedded VM     | just like [minikube](https://kubernetes.io/docs/setup/learning-environment/minikube/) |
| Python only     | [contribute_models_new.md](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/contribute_models_new.md) |

## Some Core Considerations From Model Contributors

1. The design should work on every desktop OS: macOS, Windows, and Linux.
1. Learn how to use Docker/VM is hard.
1. Writing Python code and SQL statements are simple.


Note that if we use Docker/VM method, we have to find a way to let user mount their repo directory into the Docker container or VM so that the model contributor can debug and test newly written models. And this is always hard for model contributors to understand what "mount" is and why we should do that. Even if we use an embedded VM like minikube, the model contributor still needs to manually specify the local directory to mount into the VM, e.g. `minisqlflow start -mount /path/to/local/repo:/path/to/vm/dir`.

The most straight forward method is that the model developers simply run `pip install sqlflow` and write model using Python then test the model using `sqlflow.train()`. And, to test using the Python API `sqlflow.train()` is very close to directly execute an SQLFlow statement `SELECT ... TO TRAIN ...` since for each SQLFlow workflow step, it parses the statement and generates code to call `sqlflow.train` after refactor.

## Conclusion

We prefer the [Python only method](https://github.com/sql-machine-learning/sqlflow/blob/develop/doc/design/contribute_models_new.md) for model contributors to develop and test SQLFlow models, to give the best experience to model contributors.
