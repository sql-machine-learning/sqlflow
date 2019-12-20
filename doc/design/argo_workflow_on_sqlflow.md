# Argo Workflow on SQLFLow

## Motivations

SQLFlow translates a SQL program, perhaps with extended SQL syntax for AI, into a workflow. Argo is a Kubernetes native workflow engine. When deploying SQLFlow on Kubernetes, SQLFlow leverages Argo to do workflow management.

When SQLFLow server receives a gRPC `Run` request that contains a SQL program, it:

1. Translates the SQL program to an Argo workflow YAML file.
1. Submits the YAML file to Kubernetes and receives an Argo workflow ID. `<---(1)`
1. Returns the workflow ID as Job ID to the client.

When SQLFLow server receives a gRPC `Fetch` request contains a job ID, it:

1. Looks up the associated Argo workflow and fetches most recent logs. `<---(2)`
1. Returns the fetched log `content` and `offset` as `FetchResponse` to the client.

The package `pkg/argo` contains two functions `Submit` and `Fetch` corresponding to the above steps marked (1) and (2) respectively.

We expect the user to call `Submit` once then keep calling `Fetch` until completion. For example,

```go
func WorkflowDemo() {
  wfJob := Submit(argoYAML)
  req := newFetchRequest(wfJob)
  for {
    res := Fetch(req)
    fmt.Println(res.Logs)
    if isComplete(res) {
      break
    }
    req = res.NewFetchRequest
    time.Sleep(time.Second)
  }
}
```

## API Design

In `pkg/argo`, we design `Submit` as `Submit(argoYAML string) *Job`, where

- `argoYAML` describes the Argo workflow in YAML format.
- `Job` contains the Argo workflow ID. The function calls `kubectl create` to submit the workflow, captures its standard output, and extracts the workflow ID.

For example, when submitting the [argoYAML](https://github.com/argoproj/argo/blob/master/examples/steps.yaml), the function returns `steps-xxxx` as the workflow ID.

We design `Fetch` as `Fetch(request FetchRequest) FetchResponse` where

- `FetchRequest` contains the workflow ID and the current fetching status.
- `FetchResponse` contains the most recent logs and refreshed `FetchRequest`.

## Implementation

### Submit API

We implement the `Submit` API as simple as the following logic:

``` go
func Submit(argoYAML string) *Job {
  workflowID, err := createWorkflowFromYAML(argoYAML)
  return newJob(workflowID)
}
```

where

- `createWorkflowFromYAML` calls [Kubernetes Create Resrouce API](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#resource-operations) to submit the workflow YAML file.

- `newJob` packages the `workflowID` into a `Job` structure:

    ``` protobuf
    message Job {
      string id = 1;
    }
    ```

### Fetch API

We implement the `Fetch` API as the following pseduo-code:

``` go
func Fetch(req *FetchRequest) *FetchResponse {
  // kubectl get pod ...
  pod := getPod(req)

  // kubectl get workflow ...
  wf := getWorkflow(req)

  logs, newLogOffset, isFinished := fetchPodLogs(pod, req.logOffset)

  newReq := newFetchRequest(wf, req.stepId, newLogOffset, isFinished)

  return newFetchReponse(logs, newReq)
}
```

where

- the `FetchRequest` and `FetchResponse` structure would as the following:

    ``` protobuf
    message FetchRequest {
      Job job = 1;
      string step_id = 2;       // fetching step id in workflow
      string log_offset = 3;    // fetching logs offset
      bool finish_fetching = 4; // True if fethcing is completion
    }

    message FetchResponse {
      message Logs {
        repeated string content = 1;
      }
      FetchRequest new_request = 1;
      Logs logs = 2;
    }
    ```

- `getPod` calls [Kubernetes Read Pod API](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#read-61) to read the specified Pod resource.
- `getWorkflow` calls [Kubernetes Read Resource API](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#resource-operations) to read the specified Argo workflow resource which is a CRD on Kubernetes.
- `getPodAndLogOffset` calls [Kubernetes Read Pod Logs API](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#read-log) to fetch `logs` and latest `offset` as following pseudo-code:

    ``` go
    func fetchPodLogs(pod *Pod, logOffset string) (string, []string, bool) {
      // kubectl logs podName --timestamps=true --since-time=logOffset
      logs := k8s_cli.logs(pod.Name, since_time=logOffset)

      //getOffsetAndContentFromLogs the last timestamp as the offset.
      //For an example of the returned logs:
      //
      //  2019-12-05T05:04:36.478475318Z  hello1
      //  2019-12-05T05:04:37.834142557Z  hello2
      //  2019-12-05T05:04:38.862899731Z  hello3
      //
      //Returns:
      //  logOffset:= 2019-12-05T05:04:38.862899731Z,
      //  logContent := []string{"hello1", "hello2", "hello3"}
      newOffset, logContent := getOffsetAndContentFromLogs(logs)

      finishFetchCurrentPod := false
      if isPodComplete(pod) && newOffset == logOffset {
        finishFetchCurrentPod = true
      }
      return newOffset, logContent, isFinishFetchCurrentPod
    }
    ```

- `updateFetchRequest` returns a new `FetchRequest` according to the current fetch status, the pseudo-code is as follows:

    ``` go
    func newFetchRequest(wf *Workflow, stepId, newLogOffset string, finishFetchCurrentStep bool) *FetchRequest {
      nextStepId := stepId
      finishFetching := false // True if fetching is complete

      if finishFetchCurrentStep {
        nextStep = getNextStep(wf, stepId)
        // no next step
        if nextStep == "" {
          finishFetching = true
        }
      }

      return &FetchRequest{
        job             : req.Job,
        stepId          : nextStep,
        logOffset       : newLogOffset,
        finishFetching  : finishFetching,
      }
    }
    ```
