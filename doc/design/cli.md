# SQLFlow Command Line Interface

## Overview

At the moment, users can build SQLFlow into a gRPC server or a [command line binary](../run/repl.md).  Both forms statically link to the SQLFlow core code and inherit all dependencies, including TensorFlow, various database client libraries, and various SQL dialect parsers in the form of gRPC backend servers. The many dependencies require us to release and run both forms in Docker containers.

For convenient deployment, we want to run the SQLFlow server in containers, and end users access the server via the command-line client. In particular, we want it easy to install the client tool, so we need to prune the dependencies. As a solution, we make the client tool a statically linked Go binary that remotely calls the SQLFlow server, so users can just download and run it.

The command-line client tool, named `sqlflow`, is complementary to other forms of clients, like Jupyter Notebook. Users can write shell or Python scripts calling the client tool to realize complex applications.
## Design

### Naming

Currently, the command-line binary form of SQLFlow has the name `repl`, which is the name of a user interface design philosophy. Many command-line tools, including mysql and python, implement the UI of REPL. Let's follow the convention of `mysql` and `mysqld` to name the new client `sqlflow`.


### Dependencies and Installation

The original `repl`'s only dependency is the `it2check` bash script that determines whether to call `sixel` to render images. We'll find a way to show images later. At the moment, we simply remove this dependency.

We'll provide installation guides on Mac and Windows to install `sqlflow` swiftly. For example:

1. Mac/Linux users can run the following command in a terminal to use `sqlflow`:
```bash
wget https://raw.githubusercontent.com/sql-machine-learning/.../sqlflow && chmod +x ./sqlflow
```

1. PC users can just click the download link to use `sqlflow`.

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

1. We'll define a new flag `--sqlflow_server` that holds the `sqlflowserver` address.

#### Configuration
We still use `godotenv` as well as environment variables to hold flags that change infrequently. The two necessary config entries are:

1. `SQLFLOW_DATASOURCE` the alternative of `--datasource`. We already have this.

1. `SQLFLOW_SERVER` the alternative of `--sqlflow_server`. This is to be added.

#### [RunSQLProgramAndPrintResult](https://github.com/sql-machine-learning/sqlflow/blob/be7c5728f47e8d3b81893c1d712974a6ddcd5f1c/cmd/repl/repl.go#L175)
 
We have to replace the call to `step.RunSQLProgramAndPrintResult` with a new function `RunSQLProgram` that would be defined in the `sqlflow` codebase. The new function should:

1. Construct a client stub from `--sqlflow_server`. 

1. Use the stub to send the SQL statement get from `ReadStmt` to the server.

1. Parse the result returned from the SQLFlow server.
    - The server side returns both `Plotille` ASCII figures and `PNG`s to the client. It up to the client code to determine how to display the figures. See https://github.com/sql-machine-learning/sqlflow/blob/be7c5728f47e8d3b81893c1d712974a6ddcd5f1c/pkg/step/step.go#L69-L85

1. Both the client construction code and result parsing code can be implemented by referring to the existing code in `sqlflowserver/main_test.go`.

#### Auto-complete
The original implementation of auto-complete is based on the `pkg/sql/attribute` package, in a pure client as `sqlflow`, we can only depend on such packages partly because the client may have a different version with the server-side.

To solve this problem, the server protocol should implement a new RPC method to pass the auto-completion dictionaries. The `main` function of `sqlflow` would call this new method to get the dictionaries.

1. The dictionaries for stable models still link the `attribute` package.
    - `attribute.PremadeModelParamsDocs` for canned estimators and `XGBoost`
	- `attribute.XGBoostObjectiveDocs` and `attribute.OptimizerParamsDocs` for TensorFlow optimizers and XGBoost objectives

2. The dictionaries for volatile model packages will be defined in a repeated field in the response message.
    - Models from `sqlflow_models`.
	- Models from SQLFlow Model Zoo.
	- This would be considered later.

#### User authentication
We don't have an authentication mechanism now. Details in this part are omitted and should be supplied by future studies.

There're two problems that should be considered seriously:

1. Leave an interface for SSO that's accepted by most companies/organizations.

1. How to safely access SQLFlow service deployed in internal networks.
