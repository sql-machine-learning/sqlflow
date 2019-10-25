# Highly Available SQLFlow Server

## Motivations

In current implementation, the SQLFlow client submits a SQL program to the SQLFlow server via a gRPC call. The client keeps the gPRC call until the completion of the SQL program. The SQLFlow server translates the received SQL program to a series of submitter programs, and runs the submitter program on the server. 

This implementation has the following pitfalls.

1. Timeout: the SQL program might be a long running job, and the on holding gRPC connection will timeout.
1. Job Persistence: the SQLFlow server owns the running state of SQL program. If the SQLFlow server fails, the SQL program also fails.
1. Job Isolation: different clients shares the same SQLFlow server, one client can affect the other.

In this design, we propose solve these pitfalls.

1. Timeout: instead of using gRPC long connections, the SQLFlow client communicates with the SQLFlow server in a polling manner.
1. Job Persistence&Isolation: instead of running submitter program locally, SQLFlow server launches the SQL job on Kubernetes via Argo.

## High-Level Design

The high-availabe SQLFlow job workflow is as follows:

<img src="figures/cluster_job_runner.png">

1. SQLFlow client sends the SQL statement via a gRPC call to the SQLFlow server.
1. For the `LocalJobRunner`:
    1. SQLFlow server launches a SQL job on the host and generates a job ID that identifies the SQL job.
    1. SQLFlow server maintains a mapping from job ID to the SQL job.
    1. SQLFlow server returns the job ID to the client.
1. For the `KubernetesJobRunner`:
    1. SQLFlow server launches a Argo workflow via Kubernetes API and executes the SQL job as a workflow.
    1. SQLFlow server fetches the Argo workflow ID as the job ID.
    1. SQLFlow server returns the the job ID to the client.
1. The SQLFlow client fetches the job result and job status in a polling manner until the returned job status is **SUCCESS** or **FAILED**.

## Proposal Details

### SQLFlow Client

The client calls `Run` and receives a string job ID. The client subsequently fetches the result using the job ID periodically. And the client is unaware of the deployment type of the server.

```python
job_id = client.Run("sqls ...")
while True:
    responses = client.Fetch(job_id)
    for r in responses.response:
        # do stuff: either print logs or construct Rows
    if responses.job_status in (SUCCEEDED, FAILED):
        break
    sleep(some_time)
```

And the Protocol Buffer definition looks like the following.

```proto
service SQLFlow {
    // Client calls Run and receives a string job ID.
    rpc Run (Request) returns (JobID);
    // Client subsequently fetches the result using the job ID periodically.
    rpc Fetch(JobID) returns (Responses);
}

message JobID {
    string job_id = 1;
}

message Session {
    string token = 1;
    string db_conn_str = 2;
    bool exit_on_submit = 3;
    string user_id = 4;
}

message Responses {
    JobStatus job_status = 0;
    repeated Response response = 1;
}

message JobStatus {
    enum Code {
        PENDING = 0;
        RUNNING = 1;
        SUCCEEDED = 2;
        FAILED = 3;
        UNKNOWN = 4;
    }
    string message = 0;
}
```

### JobRunner Interface

The `JobRunner` interface should provide two functions `run` and `fetch`:

```go
type JobRunner interface {
  run(sql *req.Sql, pr *PipeReader, pw *PipeWriter) (jobID string, err error){
  fetchResult(jobID string) (responses *pb.Responses)
}
```

Registe `JobRunner` in `sql.Server`:

```go
func (s *Server) Run (ctx context.Context, req *pb.Request) (*pb.JobID, error) {
  db := s.db
  pr, pw := sf.Pipe()
  jobID := s.jobRunner.run(req.Sql, pr, pw)
  return &pb.JobID{jobID: jobID), nil
}

func (s *Server) Fetch (ctx context.Context, jobID *pb.JobID) (*pb.Responses, error) {
  responses, error := s.jobRunner.fetch(jobID.jobID)
  return responses, nil
}

func main() {
  // registe `LocalJobRunner` or `KubernetesJobRunner` according to the env variable `SQLFLOW_JOB_RUNNER`
  server.RegisteJobRunner(os.getenv("SQLFLOW_JOB_RUNNER"))
}
```

### LocalJobRunner

Upon processing a `Run` request, the server generates, bookkeeps, and returns the job ID to the client.
Upon processing a `Fetch` request, the server looks up the result channel and returns the most recent result.

```go
type LocalJobRunner {
  jobs map[string]*PipeReader
}

func (r *LocalJobRunner) run(sql *req.SQL, pr *PipeReader, pw *PipeWriter) (string, error){
  jobID := jobIDGen()
  r.jobs[jobID] = pr
  go func() {
      defer pw.Close()
      pw.Write(`RUNNING`)
      sqlStatements, _ := sf.SplitMultipleSQL(sql)
      for _, singleSQL := range sqlStatements {
         for e := range s.run(singleSQL, db, s.modelDir, req.Session).ReadAll() {
            pw.Write(e)
         }
      }
      pw.Write(`SUCCEEDED`)
  }()
  return jobID, nil
}

func (r *LocalJobRunner) fetch(jobID string) (*pb.Responses, error) (
  responses := &pb.Responsts{}
  pr, ok := r.jobs[jobID]
  if !ok {
      return nil, fmt.Errorf("unrecognized jobID %s", jobID)
   }
   for ;; {
      select {
      case res := <-pr.ReadAll():
         // construct result
      case <-time.After(1 * time.Second):
         break;
      }
   }
   return responses, nil
)

```

Since we have multiple gRPC calls for a server instance, we need to maintain a state across different calls.
So we use a map `map[string]*PipeReader` to maintain the job states on the server

### KubernetesJobRunner

Upon processing a `Run` request, the server launches a Kubernetes Pod and returns the Pod ID and Argo UI URL.
Upon processing a `Fetch` request, the server checks the Pod status and returns the `JobStatus` and logs.

```go
type KubernetesJobRunner {
}

func (r *KubernetesJobRunner) run(sql *req.Sql, pr *PipeReader, pw *PipeWriter) (string, error){
  // codegenArgo generates Argo YAML file from the input SQL program.
  workflow, err := codegenArgo(sql)
  // submit the Argo workflow to the Kubernetes cluster.
  jobID, err := r.submitArgoWorkflow(workflow)
  pw.Write(fmt.Sprintf("Argo UI URL: %s", r.argoUI(jobID)))
  return jobID, nil
}

func (r *KubernetesJobRunner) fetch(jobID string) (*pb.Result, error) (
  responses := &pb.Responses{}
  responses.job_status := r.workFlowStatus(jobID)
  logs := r.lastStepLogs(jobID)
  // construct logs message ...
  return responses, nil
)
```

`codegenArgo` generates an Argo multi-steps workflow from the input SQL program. Each step would execute a
single SQL statment.

``` yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: steps-
spec:
  entrypoint: hello-hello-hello

  templates:
  - name: hello-hello-hello
    # Instead of just running a container
    # This template has a sequence of steps
    steps:
    - name: sql1  # sql1 is run before the following steps
      container:
        image: sqlflow/sqlflow 
        command: [sqlflowcmd]
        args: ["-e", "sql1..."]
    - name: sql2 # sql2 run after previous step
      container:
        image: sqlflow/sqlflow 
        command: [sqlflowcmd]
        args: ["-e", "sql2..."]
```

### Dealing with failures

Q: What if the client forgets to fetch? For example, a user hits `ctrl+C` in the Jupyter Notebook right after the `client.Run`.

A: The client can specify a timeout T. The server will kill the corresponding job if the client doesn't fetch in the last T seconds.
