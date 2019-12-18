# Argo Workflow on SQLFLow

## Overview

SQLFlow translates a SQL program, perhaps with extended SQL syntax for AI, into a workflow. Argo is a Kubernetes native workflow engine. When deploying SQLFlow on Kubernetes, SQLFlow leverages Argo to do workflow management.

When SQLFLow server receives a gRPC `Run` request that contains a SQL program, it:

1. Translates the SQL program to an Argo YAML file.
1. Submits the YAML file to Kubernetes and receives an Argo workflow ID.(1)
1. Returns the workflow ID as a job token to the client.

When SQLFLow server receives a gRPC `Fetch` request contains a job token, it:

1. Looks up the associated Argo workflow and fetches most recent logs.(2)
1. Returns the fetched logs with a updated job token to the client.

The (1) and (2) are implemented by `pkg/argo` as `Submit` and `Fetch` respectively.

## API Design

In `pkg/argo`, we design `Submit` as `Submit(argoYAML string) Job`, where

- `argoYAML` describes the Argo workflow in YAML format.
- `Job` contains the Argo workflow ID. The function calls `kubectl create` to submit the workflow, captures its standard output, and extracts the workflow ID.

For example, when submitting the [argoYAML](https://github.com/argoproj/argo/blob/master/examples/steps.yaml), the function returns `steps-xxxx` as the workflow ID.

We design `Fetch` as `Fetch(token FetchToken) FetchResponse` where

- `FetchToken` contains the workflow ID and the current fetching state.
- `FetchResponse` contains the most recent logs and refreshed `FetchToken`.

We implement the `Fetch` API as the following logic:

``` go
func Fetch(token FetchToken) *FetchResponse {
  pod, logOffset := getPodAndLogOffset(token)

  logs, newLogOffset, isFinished := FetchPodLogs(pod, logOffset)

  newToken := updateToken(token, newLogOffset, isFinished)

  return newFetchReponse(logs, newToken)
}
```

## Usage

We expect the user to call `Submit` once then keep calling `Fetch` until completion. For example,

```go
func WorkflowDemo() {
  wfJob := Submit(argoYAML)
  token := newFetchToken(wfJob)
  for {
    res := Fetch(token)
    fmt.Println(res.Logs)
    if isComplete(res) {
      break
    }
    token = res.NewToken
    time.Sleep(time.Second)
  }
}
```
