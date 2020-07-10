# SQLFlow Code Generator

SQLFlow is a compiler that takes a SQL program and generates Argo workflow, which is a `.YAML` file.
The Argo controller running on Kubernetes is the executor that executes the workflow. This is a design doc on how to implement
the backend of the SQLFlow compiler.

## The High-level Design of the Code Generator

As mentioned above, SQLFlow compiler generates the `.YAML` file as the following, you can check more detail
about SQLFlow workflow from [here](/doc/design/workflow.md).

``` yaml
steps:
    name: step-1
    command: ["python", "-c"]
    args: |
        from runtime import tensorflow
        tensorflow.train(....)
    env:
      name: SQLFLOW_OSS_AK
      value: "xxxxxx"
```

From the above workflow `.YAML` file, each workflow step contains three parts:

1. The execution command as the `command` spec to execute the program.
1. The execution program called it the submitter program, which could be written in Python, R, or Bash programming language.
The submitter program submits an AI task on an AI platform e.g., [ElasticDL](https://github.com/sql-machine-learning/elasticdl), [Alibaba PAI](https://www.alibabacloud.com/help/zh/doc-detail/75093.htm), or just running on host via `runtime` library.
1. The runtime environment variables with the `env` spec.

SQLFlow compiler provides the code generator component to generate the step program,
the code generation is divided into the following stages:

1. [Target Submitter Registry](#target-submitter-register), register a Code Generator in SQLFlow compiler.
1. [CodeGenerator Interface](#code-generator-interface) is a Go interface that all code generators should implement.
1. [Code Generation](#code-generation) provides an assembler API to generate a step program.

### Target Submitter Register

For a new code generator, develops should register it in SQLFlow compiler as the following pseudo-code:

``` golang

cgMapping = map[string]CodeGenerator {
  "paiTensorFlow": PAITensorFlow{},
  "paiXGBoost", PAIXGBoost{},
  ...
}
```

### Code Generator Interface

For each code generator implementation, you should care about all IR types, different IR types have different behaviors and
generate different submitter program. Each code generator owns an `ExecutionCtx` instance to tell Argo workflow
on how to execute the target code.

``` golang
type ExecutionCtx struct {
  ExecCommand []string      // How to execute the target code, .e.g ["python" "-c"]
  Env map[string]string     // The environment variables for execution
}

type CodeGenerator interface {
  ExecCtx(*ir.SQLStmt) ExecutionCtx
  EmitNormal(*ir.NormalStmt) (string, error)
  EmitTrain(*ir.TrainStmt) (string, error)
  EmitPredict(*ir.PredictStmt) (string, error)
  EmitExplain(*ir.ExplainStmt) (string, error)
  EmitEvaluate(*ir.EvaluateStmt) (string, error)
  EmitShowTrain(*ir.ShowTrainStmt) (string, error)
  EmitOptimize(*ir.OptimizeStmt) (string, error)
  EmitRun(*ir.RunStmt) (string, error)
}
```

### Code Generation

The code generation phase is responsible for generating target code from a SQL statement IR, this is an
assembler API that routes to a specified code generator, the pseudo-code is as the following:

``` golang
func Generate(session *pb.Session, stmt *ir.SQLStatement) (string, error) {
  // routing to a specified code generator from session.submitter
  cf := cgMapping[session.submitter]
  
  switch v := stmt.(type) {
  case *ir.TrainStmt:
    return cg.Train(stmt.(*ir.TrainStmt)), cg.ExecutionCtx, nil
  case *ir.PredictStmt:
    return cg.Predict(stmt.(*ir.TrainStmt)), cg.ExecutionCtx, nil
  ...
  }
}

```