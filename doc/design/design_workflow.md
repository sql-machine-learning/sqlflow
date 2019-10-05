# Workflow

## Motivation

SQLFlow translates a SQL program, perhaps with extended SQL syntax for AI, into a workflow. Currently, it translates a SQL program into a Python program.

Programming languages are a natural solution to workflow description. However, we want another way of workflow description, because we want to force users to explicitly declare steps and their dependencies. Given such information, we could identify shared parts between workflows, which might come from a user or different users, and remove redundent executions of the shared parts.

A secondary motivation is to identify steps that could run simultaneously from dependency analysis. Or, in short, to improve concurrency of the execution of a workflow.

## Related Work

### Programming Languages

Programming language are the most intuitive way to describe a workflow. And, they can describe pretty complex workflows. The minimum computational unit in proramming langauges is some primitive built-in functions and operators.  For AI system developers, most workflows consist of steps with a certain granularity -- a job running on Kubernetes.  It is true that the language runtime, i.e., the compilers and interpreters, often optimzie the exeuction of the "workflow" by concurrently running primitives, but they don't parallelize jobs.

We prefer the intuitive description of workflows provided by programming languages, but we need to define a certian granulairty as steps.

### TensorFlow

TensorFlow is a deep learning system, which allows users to describe a computaiton process known as the *forward pass*, and runs an algorithm kownn as *autodiff* to derive the *backward pass* automatically from the forward pass.

TenosrFlow 1.x represents the computation process by a data structure known as a *graph*, whose each node is a step, known as a *TensorFlow operation*.

We prefer the workflow engine represents each workflow as a data structure or something similar, so we can identify shared parts among multiple graphs, and merge multiple workflows into a big one without redundent parts.

### Google Tangent

Tangent is another deep learning system developed by Google. In contrast to TensorFlow, it represents the computation process by Python source code, other than a graph.

Tangent does autodiff by parsing the Python source code into an abstract syntax tree (AST), adding the backward steps into the AST tree, and printing the AST tree into a new snippet of Python code.

We like the capability of describing the computation process by a program.  Tangent doesn't support all Python syntax used in the description of the forward pass. Similarly, we might not allow all Python syntax used to describe the workflow, if we follow the Tangent way.  The steps in Tagnent include some pre-listed functions, mostly, TensorFlow operations, and Python operators.

## Concepts




What is workflow?

**What are included in a workflow specification?**
workflow + step

**How to define a workflow step?**
Using function?

Some base jobs

WorkflowParam represents intermediate values passed between steps, and can also be used to find out dependencies. (name, owner_step, type).
How inputs/outputs are supported in Argo now?

```python
# Base job
class BaseJob(object):
    def __init__(
        self,
        name: str,
        inputs: List[WorkflowParam],
        outputs: List[WorkflowParam],
        retry: int,
        timeout: int,
    ):
    	# Create a new instance of BaseJob


# Single container job
class ContainerJob(BaseJob):
    def __init__(
        self,
        name: str,
        image: str,
        command: str,
        args: str,
        **kwargs,
    ):
    	# Create a new instance of ContainerJob
        

# A k8s resource job
class ResourceJob(BaseJob):
    def __init__(
        self,
        name: str,
        success_condition: str,
        failure_condition: str,
        **kwargs,
    ):
        # Create a new instance of ResourceJob

```

```python
# What is loop has a linear sequence inside?
@for_loop()
@while_loop()
@when(step1.output.a="abc")

# For a simple bash step
@Run_with(....)
@for_loop(....)
def print_date():
    return ContainerJob(
        name="name",
        image="",
        command="",
        args="",
    )


# Define a workflow
def create_workflow():
    ...
    return Workflow(
        name="pipeline1",
        steps=[a, b, c],
        ...
    )

ArgoRunner().run(create_workflow())
```


**Non-conditional Loop**
	Static loop: authors specify the number of iterations in the DSL code.
	Dynamic loop: Since argo supports template parameters, the number of iterations could be from the input parameters when users submit the pipeline run.

**Conditional Loop**
The loop condition is based on the runtime statuses and the downstream operators can depend on the loop as a whole and specify the I/O dependencies. However, the downstream operators can only access the output from the loop of the last iteration. 


**How to describe dependencies?**
