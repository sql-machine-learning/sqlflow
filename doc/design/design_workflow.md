# Workflow

Motivation

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
