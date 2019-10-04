# Workflow

## Motivation

Programming languages are a natural solution to workflow description. We want an API, other than programming languages, because we want to force users to explicitly declare steps and their dependencies. Given such information, we could identify shared parts between workflows, which might come from a user or different users, and remove redundent executions of the shared parts.

A secondary motivation is to identify steps that could run simultaneously from dependency analysis. Or, in short, to improve concurrency of the execution of a workflow.

## Related Work

### Programming Languages

Programming language are the most intuitive way to describe a workflow. And, they can describe pretty complex workflows. The minimum computational unit in proramming langauges is CPU instructions, which are often hidden from programmers.  For AI system developers, most workflows consist of steps with a certain granularity -- a job running on Kubernetes.  It is true that the language runtime, i.e., the compilers and interpreters, often optimzie the exeuction of the "workflow" by concurrently running CPU instructions, but they don't parallelize jobs.

We prefer the intuitive description of workflows provided by programming languages, but we need to define a certian granulairty as steps.

### TensorFlow

TenosrFlow 1.x provides a define-and-run API that describes a computation process, adding some steps into it (known as autodiff in the terminology of deep learning), then run the computation process.  TensorFlow 1.x represents the process as a data structure known as *graph*, which looks very similar to workflow.

We prefer the workflow engine represents each workflow as a data structure, so we can identify shared parts among multiple graphs, and merge multiple workflows into a big one without redundent parts.

### Google Tangent

Tangent is another deep learning system developed by Google. In contrast to TensorFlow, it represents the computation process by Python source code, other than a graph.

As a deep learning system, Tangent needs to do autodiff to add steps of the backward pass.  It parses the Python source code into an abstract syntax tree (AST) and adds the backward steps into the AST tree, then prints the AST tree into a new segment of Python code.

Google Tangent allows users to describe the deep learning computaiton process, in particular, the forward pass, in Python, which is familar to most programmers.  Such convenience is what we want.

Tangent doesn't support all Python syntax used in the description of the forward pass. Similarly, we might not allow all Python syntax used to describe the workflow, if we follow the Tangent way.

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
