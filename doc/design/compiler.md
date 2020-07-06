# SQLFlow Compiler Overview

A typical AI pipeline contains a sequence steps: data-processing, model training, model evaluating and model prediction.
SQLFlow aims to design an extended SQL syntax to abstract the end-to-end AI pipeline. Instead writing Python/R/SQL program, users
just writing SQL program to finish the AI pipeline.

## Primary Components

SQLFlow is a compiler that translate user typed SQL program into a workflow, each workflow step
submit a standard SQL statement to DBMS, or submit a AI application to AI platform e.g. EDL, Alibaba PAI or just local running.

Just like GCC translate C/C++ program into .. running on Linux operation system, SQLFlow translate SQL program into
workflow which is a `.YAML` file running on a distributed operation system -- Kubernetes.

The primary components of SQLFlow compiler is as the following.

1. [Parser](#Parser) is the frond-end of SQLFlow compiler, which parsing user typed SQL program into an Intermediate Representation(IR).
1. [Sematic Analyze](#sematic-analyze) check the attribute and fill the IR.
1. [Optimizer](#optimizer) is the middle-end of SQLFlow compiler.
1. [Code Generator](#code-generator) is the back-end of SQLFlow compiler, which takes the IR and outputs Argo workflow with a `.YAML` file.
1. [Runtime Library](#runtime-library) provides a library that running in each workflow steps.

### Parser

SQLFlow not only supports standard SQL syntax, but also the extended SQL syntax followed by `SELECT` clause, e.g. `SELECT ... TO TRAIN ...` to train a model,
`SELECT ... TO PREDICT ... USING ...` to predict using a trained model and many more on
[SQLFlow Language Guide](https://sql-machine-learning.github.io/sqlflow/doc/language_guide/)
For the standard SQL syntax, users may use various SQL engine .e.g. MySQL, Hive and Alibaba MaxCompute.
SQLFlow parser component implemented [collaboribe parsing](https://arxiv.org/pdf/2001.06846.pdf) algorithm to parse standard SQL syntax and
extended SQL syntax. The [Parser](/go/parser/sqlflow_parser.go) package provides an interface as the following.

``` golang
// Parse a SQL program in the given dialect into a list of SQL statements.
func Parse(dialect, program string) ([]*SQLFlowStmt, error) {
  ...
}
```

### Sematic Analyze

### Optimizer

The optimizer component takes the SQLFlow IR, then run the specified optimizations or analyzer on it, at last outputs the optimized or analyzed IR.
For example, the [SQL program dependency analyzer](doc/design/sql_program_dependency_analyze.md) can analyze the dependency between SQL
statements, that Argo controller on a Kubernetes cluster would execute the generated workflow with `.YAML` file in parallel as much as possible.

### Code Generator

SQLFlow compiler contains two-layers code generators.

1. One generates Argo workflow which is `.YAML` file.
1. Another takes a SQL statement IR and generates the submitter program to fill the `.YAML` file, the submitter program would be a Python/R/Bash script.

The piece of the `.YAML` file is as the following:

``` yaml
steps:
    name: step-1
    args: ["python", "-c"]
    command: |
        import sqlflow.runtime.tensorflow
        tensorflow.train(....)
```

SQLFlow develops implements the following `CodeGenerator` Go interface to generate a submitter program.

``` golang

type SubmitterProg struct {
  execCmd []string    // the execution command to execute the generated program, e.g. ["python" ,"-c"], ["bash" ,"-c"]
  program string      // the generated submitter program
}

type CodeGenerator interface {
  Query(*ir.NormalStmt) (SubmitterProg, error)
  Train(*ir.TrainStmt) (SubmitterProg, error)
  Predict(*ir.PredictStmt) (SubmitterProg, error)
  Explain(*ir.ExplainStmt) (SubmitterProg, error)
  Evaluate(*ir.EvaluateStmt) (SubmitterProg, error)
  ShowTrain(*ir.ShowTrainStmt) (SubmitterProg, error)
  Optimize(*ir.OptimizeStmt) (SubmitterProg, error)
  Run(*ir.RunStmt) (SubmitterProg, error)
}
```

### Runtime Library

## SQLFlow Compiler Toolkit

To make SQLFlow compiler works well with other system, SQLFlow project provides many toolkit.

### SQLFlow gRPC Server

### SQLFlow Language Server

### Command-line Tool
