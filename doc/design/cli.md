# SQLFlow Command Line Interface

## Overview

At the moment, users can build SQLFlow into a server or a [command line binary](../run/repl.md). Both of the executables have all the functionalities of SQLFlow, and both of them can only run in a Docker container.

For a deployment of SQLFlow in a production environment, the SQLFlow server runs in a Kubernetes cluster. Users access SQLFlow from a Web IDE or the Jupyter Notebook.

For an individual developer, one may deploy SQLFlow on her personal computer as described in [Run SQLFlow Using Docker](../run/docker.md)

For the above two typical scenarios, SQLFlow lacks an easy-to-use user interface for quick start, quick verification, or quick deployment. Jupyter Notebooks and web IDEs still have a certain learning curve that reduces the user experience of SQL programmers. 

From another point of view, a CLI could overtake a GUI in user experience for a simple language as SQLFlow: it's easier to design, easier to implement, easier to obtain. For example, repetitive tasks can be simplified by line editing and history mechanisms. With a CLI program, a user can easily access all the power of SQLFlow from a terminal as:

```bash
$ sqlflow
sqlflow> _
```

To achieve this goal, we also want to hide implementation details about Docker because users don't have to know this. As a result, there are two possible solutions:
1. Wrap the current SQLFlow REPL in a script, we already have such an [attempt](https://github.com/sql-machine-learning/sqlflow/pull/2114).
1. Decouple the REPL implementation from the core code of SQLFlow and provide a native binary that only functions as a pure client.

We decide to choose the second solution because it's more lightweight, if a user already has a SQLFlow deployment in her company, all she has to do is to download the native binary.

## Design

### Naming

`REPL` is not a good name, We'll rename the binary as simply `sqlflow`.

### Dependencies and Installation

The original `repl`'s only dependency is the `it2check` bash script that determines whether to call `sixel` to render images.

We'll provide installation scripts on Mac (Bash) and Windows (PowerShell) to install `sqlflow` and its dependencies swiftly. The four scripts are:

1. install.sh (Install Docker, `sqlflow`, and the image `sqlflow/sqlflow:latest` on a Mac)
1. install-client.sh (Only `sqlflow` on a Mac)
1. install.ps1(Install Docker, `sqlflow`, and the image `sqlflow/sqlflow:latest` on a PC, coming later)
1. install-client.ps1 (Only `sqlflow` on a PC, coming later)

### Refactoring

Fortunately, there'are only a few places in the REPL codebase that requires SQLFlow core.

#### Workflow

At the moment, SQLFlow workflow calls `repl` to execute a single step, after the refactoring, as `repl` has become a pure client, the workflow code should call the [`step` binary](https://github.com/sql-machine-learning/sqlflow/blob/be7c5728f47e8d3b81893c1d712974a6ddcd5f1c/cmd/step/step.go#L45-L53) instead.

Consequently, we can delete code in `repl` about workflow safely:

1. [`isWorkflowStep`](https://github.com/sql-machine-learning/sqlflow/blob/be7c5728f47e8d3b81893c1d712974a6ddcd5f1c/cmd/repl/repl.go#L314-L320)

1. [Table in protobuf](https://github.com/sql-machine-learning/sqlflow/blob/be7c5728f47e8d3b81893c1d712974a6ddcd5f1c/cmd/repl/repl.go#L161-L163)

1. [The step package](https://github.com/sql-machine-learning/sqlflow/blob/be7c5728f47e8d3b81893c1d712974a6ddcd5f1c/cmd/repl/repl.go#L35)

#### [Command Line Flags](https://github.com/sql-machine-learning/sqlflow/blob/be7c5728f47e8d3b81893c1d712974a6ddcd5f1c/cmd/repl/repl.go#L261-L268)

1. We don't need the flag `--model_dir` any more because the training process is running in `sqlflowserver`.

1. We'll define a new flag `--host` that holds the `sqlflowserver` address.

#### Configuration
We still use `godotenv` as well as environment variables to hold flags that change infrequently. The two necessary config entries are:

1. `SQLFLOW_DATASOURCE` the alternative of `--datasource`. We already have this.

1. `SQLFLOW_SERVER_HOST` the alternative of `--host`. This is to be added.

#### [RunSQLProgramAndPrintResult](https://github.com/sql-machine-learning/sqlflow/blob/be7c5728f47e8d3b81893c1d712974a6ddcd5f1c/cmd/repl/repl.go#L175)
 
We have to replace the call to `step.RunSQLProgramAndPrintResult` with a new function `RunSQLProgram` that would be defined in the `sqlflow` codebase. The new function should:

1. Construct a client stub from `--host`. 

1. Use the stub to send the SQL statement get from `ReadStmt` to the server.

1. Parse the result returned from the SQLFlow server.
    - For non-sixel terminal simulators, the server side should return `Plotille` ASCII figures. See https://github.com/sql-machine-learning/sqlflow/blob/be7c5728f47e8d3b81893c1d712974a6ddcd5f1c/pkg/step/step.go#L69-L85

1. Both the client construction code and result parsing code can be implemented by referring to the existing code in `sqlflowserver/main_test.go`.

#### Auto-complete
The original implementation of auto-complete is based on the `pkg/sql/attribute` package, in a pure client as `sqlflow`, we can only depend on such packages partly because the client may have a different version with the server-side.

To solve this problem, the server protocol should implement a new RPC method to pass the auto-completion dictionaries. The `main` function of `sqlflow` would call this new method to get the dictionaries.

1. The dictionaries for stable models have their own fields in the response message.
    - `attribute.PremadeModelParamsDocs` for canned estimators and `XGBoost`
	- `attribute.XGBoostObjectiveDocs` and `attribute.OptimizerParamsDocs` for TensorFlow optimizers and XGBoost objectives

2. The dictionaries for volatile model packages are in a repeated field in the response message.
    - Models from `sqlflow_models`.
	- Models from SQLFlow Model Zoo.

#### User authentication
We don't have an authentication mechanism now. Details in this part are omitted and should be supplied by future studies.

There're two problems that should be considered seriously:

1. Leave an interface for SSO that's accepted by most companies/organizations.

1. How to safely access SQLFlow service deployed in internal networks.
