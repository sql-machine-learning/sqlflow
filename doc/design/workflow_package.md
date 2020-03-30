# Workflow Package

## Motivation

SQLFlow translates a SQL program, perhaps with extended SQL syntax for AI, into a workflow. Tekton/Argo are Kubernetes native workflow engine when deploying SQLFlow on Kubernetes, SQLFlow leverages Argo/Tekton to do the workflow management.

SQLFlow supports Argo/Tekton as the workflow backend and maybe more in the future. It's different to communicate with the theses workflow engine, they are different CRD on Kubernetes, and they have different YAML spec, so it's necessary to organize a separate package `workflow` to communicate the workflows with an uniform interface.

## Design

To implement the above motivation, the `workflow` package should include the following functionalities:

1. `CodeGen`: Go interface to generate the Fluid/Argo program to generate workflow YAML.
1. `Submit/Fetch`: APIs to submit the workflow and trace the status of the workflow step.

We propose the following code structure:

``` text
workflow/
        |-workflow.go           # workflow interface
        |-argo/                 # submit/trace argo workflow via k8s API
        |-tekton/               # submit/trace tekton workflow via k8s API
        `-codegen/
                |-fluid/        # generate Tekton YAML using Fluid
                `-couler/       # generate Argo YAML using Couler
```

### Workflow Codegen

Couler/Fluid lets users write Argo and Tekton workflows in Python rather than YAML. Also, the Python code is easier to read and code review.
SQLFlow implements Fluid `Codegen` to translate the `[]ir.SQLFlowStmt` into Python code, the interface can be like：

``` golang
type Codegen interface {
  GenCode([]ir.SQLFlowStmt) string
  GenYAML(string) string
}
```

- `GenCode` inputs a SQL program and outputs the Fluid program in Python.
- `GenYAML` compiles the Fluid program and outputs the workflow YAML.

### Workflow Interface

``` golang
type Workflow interface {
  Submit(yaml string) (workflowID string, err error)
  Fetch(FetchRequest) FetchResponse
}

func New(backend string) (Codegen, Workflow, error) {
  if backend == "tekton" {
    return NewCodegen("fluid"), NewWorkflow("tekton"), nil
  }
}
```

- `Submit` submits the input YAML content to a Kubernetes cluster, and returns the workflow.
- `Fetch` fetches the step status and query result which packaged in `FetchResponse`.
- `New` returns the corresponding Codegen and Workflow implementation.

### Execution Example

``` golang
// New codegen and workflow operator
cg, wf, e := workflow.New("tekton")

// generate YAML file
py := cg.GenCode(SQLProgram)
yaml := cg.GenYAML(py)

// submit the workflow YAML and retrieval workflow step status
wfID := wf.Submit(yaml)
fetchRequest := NewFetchRequest(wfID)
for {
  response := wf.Fetch(fetchRequest)
  // deal with response.Message, response.Rows, e.g.
  // Eof means the workflow completed, break the loop
  if response.Eof {
    break
  }
  fetchRequest = response.updated_request_since
}
```
