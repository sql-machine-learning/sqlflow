# Couler

Couler is a programming language for describing workflows. It shares Python's syntax, but only a small subset -- the function definition and invocation. Couler is also the name of a compiler, which translates Couler programs into [Argo](https://argoproj.github.io/) YAML files.

## Motivations

A motivation of Couler comes from the requirement of SQLFlow. SQLFlow translates SQL programs, with optionally AI syntax extension, into Couler programs, which, the Couler compiler translates into Argo YAML files.

SQLFlow needs Couler because it needs Argo. It needs Argo because it requires a workflow execution engine. It requires a workflow engine because, in most setups, the SQLFlow server cannot merely translates each SQL statement into a Python submitter program and runs them one-by-one. If it does so, the SQLFlow engine works like the workflow engine and needs to keep the status of the executions of workflows. However, unfortunately, as SQLFlow runs on Kubernetes as a service, which is the most common case, each server instance might be preempted at any time. The SQLFlow server could indeed save the status in robust storage like etcd; however, that introduces a lot of code and makes SQLFlow a duplication of reliable workflow engines like Argo.

We build Couler on top of Argo for some reasons: 

- Argo YAML is less comprehensive, and it is hard to debug if we make SQLFlow generate YAML files directly. 
- As we introduce Couler as a human-readable intermediate representation, it would benefit Python programmers in addition to SQLFlow users.

## The Design

### Steps and Functions

Couler users write a workflow as a Python program, where each step is a Python function definition, and the workflow itself is a sequence of function invocations. We want step-functions like the following.

- `couler.mysql.run(sql)`
- `couler.mysql.export_table(table, filename)`
- `couler.xgboost.train(model_def, training_data)`
- `couler.xgboost.predict(trained_mode, test_data)`

### Couler Core

When users define a step function, they could call the following fundamental functions provided by Couler.

- `couler.run_container(docker_image, cmd, args)` starts a container to run a command-line with arguments. It returns values extracted from the standard output.
- `couler.run_script(docker_image, function_name)` runs a Python function defined in the current Couler program in a container. It returns values extracted from the standard output.
- `couler.when(condition, step)` runs a step if the condition lambda returns true.
- `couler.map(step, a_list)` repeatedly runs a step for each of the value in a given Python list.

### Step Zoo

A collection of Couler step functions from a step zoo. Because each step runs in a Docker container, the step zoo might also container some Dockerfiles. It recommended configuring the CI/CD system to build the Docker images from Dockerfiles automatically.

## Argo and Docker Mode

To make the debug even comfortable, we can make Couler support a *Docker mode* in addition to the *Argo mode*.  In both ways, each step runs as a Docker container.  The difference is that the containers run on a Kubernetes cluster in the Argo mode, but on the local host computer in Docker mode.
