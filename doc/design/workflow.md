# Workflow

## Motivation

SQLFlow translates a SQL program, perhaps with extended SQL syntax for AI, into a workflow. Currently, it translates a SQL program into a Python program.

Programming languages are a natural solution to workflow description. However, we want another way of workflow description because we want to force users to declare steps and their dependencies explicitly. Given such information, we could identify shared parts between workflows, which might come from a user or different users, and remove redundant executions of the shared components.

A secondary motivation is to identify steps that could run simultaneously from dependency analysis. Or, in short, to improve concurrency of the execution of a workflow.

## Related Work

### Programming Languages

Programming languages are the most intuitive way to describe a workflow. And, they can define pretty complex workflows. The minimum computational unit in programming languages is some primitive built-in functions and operators.  For AI system developers, most workflows consist of steps with a certain granularity -- a job running on Kubernetes.  It is true that the language runtime, i.e., the compilers and interpreters, often optimize the execution of the "workflow" by concurrently running primitives, but they don't parallelize jobs.

We prefer the intuitive description of workflows provided by programming languages, but we need to define a certain granularity as steps.

### TensorFlow

TensorFlow is a deep learning system, which allows users to describe a computation process known as the *forward pass*, and runs an algorithm known as *autodiff* to derive the *backward pass* automatically from the forward pass.

TensorFlow 1.x represents the computation process by a data structure known as a *graph*, whose each node is a step, known as a *TensorFlow operation*.

We prefer the workflow engine represents each workflow as a data structure or something similar, so we can identify shared parts among multiple graphs, and merge various workflows into a big one without redundant components.

### Google Tangent

Tangent is another deep learning system developed by Google. In contrast to TensorFlow, it represents the computation process by Python source code, other than a graph.

Tangent does autodiff by parsing the Python source code into an abstract syntax tree (AST), adding the backward steps into the AST tree, and printing the AST tree into a new snippet of Python code.

We like the capability of describing the computation process by a program.  Tangent doesn't support all Python syntax used in the description of the forward pass. Similarly, we might not allow all Python syntax used to describe the workflow if we follow the Tangent way.  The steps in Tangent include some pre-listed functions, mostly, TensorFlow operations, and Python operators.

## Proposal

There seem multiple strategies to design the high-level API of a workflow engine.

1. The program written in this API runs and executes the workflow.  This way works obviously as the interpreter/runtime of the host language serves as the workflow engine.  However, we might want to use some other workflow engines, like Argo.
1. The program written in the API runs and generates the YAML workflow definition, which is the input for Argo.
1. We write a **transpiler** to convert the program in the API into the YAML workflow.

These strategies are not necessarily mutually exclusive to each other. The key depends on how we implement the [control flow](https://en.wikipedia.org/wiki/Control_flow).  If we use control flows of the host programming language, 1. and 3. work. Otherwise, we define control flows as API calls, then all of them work.

### Control Flow as API Calls

With an API function `couler.for` representing a loop, we can write the following example program.

```python
def loop_example():
    couler.for(whalesay, ["hello world", "goodbye world"])

def whalesay(message):
    couler.run_container(image="docker/whalesay:latest", command=["cowsay"], args=[message])
```

We can define `couler.run_container` to call the Docker API and run a container, and `couler.for` to call Python's loop control flow. In this way, the above program can run and execute a workflow.

Alternatively, by defining `couler.run_container` and `couler.for` in some particular way, we can make sure that when we run the above program, it generates the following YAML file.

```yaml
spec:
  entrypoint: loop-example
  templates:
  - name: loop-example
    steps:
    - - name: print-message
        template: whalesay
        arguments:
          parameters:
          - name: message
            value: "{{item}}"
        withItems:              # invoke whalesay once for each item in parallel
        - hello world           # item 1
        - goodbye world         # item 2

  - name: whalesay
    inputs:
      parameters:
      - name: message
    container:
      image: docker/whalesay:latest
      command: [cowsay]
      args: ["{{inputs.parameters.message}}"]
```

A transpiler that calls the parser of the host language (Python in this example) can do the above conversion from a program to a YAML file. However, if the program can run and generate the YAML itself, we might not bother to write the transpiler.

### Control Flow from the Host Language

Let us rewrite the above example program using Python's control flow.

```python
def loop_example():
    for m in ["hello world", "goodbye world"]:
        whalesay(m)

def whalesay(message):
    couler.run_container(image="docker/whalesay:latest", command=["cowsay"], args=[message])
```

When we run the above program, it executes the workflow and calls `whalesay` three times.

But if we want the YAML file, we would have to write a transpiler that takes the Python program as the input and converts it, in particular, the for loop, into the YAML file.

## Conclusion

Let us try the control-flow-as-API strategy first, as it seems easier to implement.  If it doesn't work, let us try the control-flow-from-host-language approach.
