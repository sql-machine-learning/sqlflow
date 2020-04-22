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

- `createWorkflowFromYAML` calls [Kubernetes Create Resource API](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#resource-operations) to submit the workflow YAML file.

- `newJob` packages the `workflowID` into a `Job` structure:

    ``` protobuf
    message Job {
      string id = 1;
    }
    ```

### Fetch API

We implement the `Fetch` API as the following pseudo-code:

``` go
func Fetch(req *FetchRequest) *FetchResponse {
  // kubectl get workflow ...
  wf := getWorkflow(req)

  // return the next step ID if necessary
  stepID := req.stepID

  // kubectl get pod ...
  pod := getPod(wf, stepID)

  // return step phase logs if the phase changed
  stepPhaseLogs := fetchStepPhaseLogs(wf, stepID)

  // append step phase status logs to Response
  responses := []pb.Response{&Message{stepPhaseLogs}}

  eof := false // True for end of job

  if podComplete(pod) {
    // if the current step execute a standard SQL, the query result would be output
    // as protobuf message string format with prefix SQLFLOW_PROTOBUF:
    //
    // SQLFLOW_PROTOBUF: head:<column_names:"col1" column_names:"col2" column_names:"col3" ... >
    // SQLFLOW_PROTOBUF: row:<data:<type_url:"type.googleapis.com/google.protobuf.DoubleValue" value:"\t\232\231\231\231\231\231\031@" >
    // ...
    responses = append(responses, unmarshalResponseFromPodLogs(pod))
    if isLastStep(wf, stepID) {
      eof = true
    } else {
      stepID = nextStep(wf, stepID)
    }
  }

  return newFetchResponse(responses, newUpdatedFetchRequest(wf, stepID), eof)
}
```

where

- the `FetchRequest` and `FetchResponse` structure would as the following:

    ``` protobuf
    message FetchRequest {
      Job job = 1;
      // the following fields keep the fetching state
      string step_id = 2;
      string step_phase = 3;
    }

    message Response {
      oneof response {
          Head head = 1;
          Row row = 2;
          Message message = 3;
          EndOfExecution eoe = 4;
          Job job = 5;
      }
    }

    message FetchResponse {
      message Responses {
        repeated Response responses = 1;
      }
      FetchRequest updated_fetch_since = 2;
      bool eof = 2;
    }
    ```

- `getPod` calls [Kubernetes Read Pod API](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#read-61) to read the specified Pod resource.
- `getWorkflow` calls [Kubernetes Read Resource API](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#resource-operations) to read the specified Argo workflow resource which is a CRD on Kubernetes.
- `fetchStepPhaseLogs` returns the format step phase logs like:

    ``` text
    Step [1/3] Execute Code: echo hello1
    Step [1/3] Log: http://localhost:8001/workflows/default/steps-bdpff?nodeId=steps-bdpff-xx1
    Step [1/3] Status: Pending
    Step [1/3] Status: Running
    Step [1/3] Status: Succeed/Failed
    Step [2/3] Execute Code: echo hello2
    Step [2/3] Log: http://localhost:8001/workflows/default/steps-bdpff?nodeId=steps-bdpff-xx2
    ```

    the log URL can be the log panel on Argo UI.

- `unmarshalResponseFromPodLogs` unmarshal protobuf message from Pod logs as following pseudo-code:

    ``` go
    func unmarshalResponseFromPodLogs(pod *Pod) ([]*pb.Response, error) {
      responses := []*pb.Response{}
      offset := ""
      logs, newOffset := k8s.readPodLogs(pod.Name, offset)
      while(!isNoMoreLogs(pod, offset, newOffset)) {
        offset = newOffset
        for _, line := range(logs) {
          // using prefix to identify the protobuf string format can avoid
          // performance degradation caused by unmarshaling all logs.
          if strings.HasPrefix(SQLFlowProtobufPrefix, line) {
            res := new(pb.Response)
            // Unmarshal the protobuf message from pod logs to Response Message
            proto.Unmarshal(line, res)
            response = append(response, res)
          }
        }
        log, newOffset = k8s.ReadPodLogs(pod.Name, offset)
      }
      return responses, nil
    }
    ```

## Pipe Message From a Workflow Step to Jupyter Notebook

``` text
SQLFLow magic command(Jupyter Notebook) <----->  SQLFlow gRPC server  <---->  Workflow Step(Kubernetes Pod)
                                          gRPC                         HTTP
```

The above figure shows the pipe stages from SQLFlow magic command to the workflow step.
SQLFlow magic command fetches the data rows and error messages via the `Fetch`
gRPC call from [Fetch API design](#Fetch-API), `Fetch` can get workflow step logs
via [Kubernetes Read Pod Logs API](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#read-log),
if the log message is in protobuf text format, `Fetch` would unmarshal and pipe it to the Jupyter Notebook.

## Retrieve Error Logs From a Failed Workflow Step

A workflow may fail for many reasons, and these errors usually occur in the following two phases:

- The first phase is compilation, e.g. syntax errors on compiling a SQL program into workflow YAML.
- The other is the workflow step runtime phase:
  - Some workflow step executes a standard SQL, the Go database driver executes the SQL and returns an error if the execution failed.
  - Some workflow step executes an extended SQL, SQLFlow compiles the extended SQL into a submitter program and executes it
    - Some submitter program would be executed as a sub-process, SQLFlow can retrieve the error logs from stderr.
    - Some submitter program would run on a cluster system e.g., Yarn, SQLFlow should retrieve the error logs from the Yarn task.

To create a good user experience, we also should pipe theses error messages to the Jupyter Notebook.
