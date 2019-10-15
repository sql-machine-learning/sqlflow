# Highly Available SQLFlow Server

## Motivations

In the current system, the SQLFlow client connects the SQLFlow server with a long live connection.
The SQLFlow client sends a gRPC request which contains a SQL statement and waits until the SQLFlow server completes executing the SQL statement.

Once the SQLFlow server receives a training SQL statement, it generates a Python training program that submits the job. This will cause:

1. The local job may cause the SQLFlow server resource insufficient when there are too many SQL jobs.
1. Sometimes, the SQL job takes too much time, and the gRPC calls timeout.
1. If one of the SQLFlow server instances fails, the SQL job also fails.

In this design, we propose to:

1. the SQLFlow client communicates with the SQLFlow server in a polling manner.
1. Implement `KubernetesJobRunner` on the server-side to launch the SQL job on Kubernetes.

We recommend using `KubernetesJobRunner` in the production environment.

## High-Level Design

The high-availabe SQLFlow job workflow is as follows:

<img src="figures/cluster_job_runner.png">

1. SQLFlow client sends the SQL statement via a gRPC call to the SQLFlow server.
1. For the `LocalJobRunner`:
    1. SQLFlow server launches a SQL job on the host and generates a job ID that identifies the SQL job.
    1. SQLFlow server maintains a mapping from job ID to the SQL job.
    1. SQLFlow server returns the job ID to the client.
1. For the `KubernetesJobRunner`:
    1. SQLFlow server launches a Kubernetes Pod via Kubernetes API and executes the SQL job in it.
    1. SQLFlow server fetches the Pod ID.
    1. SQLFlow server returns the Pod ID as the job ID to the client.
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
  run(sql string, pr *PipeReader, pw *PipeWriter) (jobID string, err error){
  fetchResult(jobID string) (responses *pb.Responses)
}
```

Registe `JobRunner` in `sql.Server`:

```go
func (s *Server) Run(ctx context.Context, req *pb.Request) (*pb.JobID, error) {
  db := s.db
  pr, pw := sf.Pipe()
  jobID := s.jobRunner.run(req.Sql, pr, pw)
  return &pb.JobID{jobID: jobID), nil
}

func (s *Server) Fetch(ctx context.Context, jobID *pb.JobID) (*pb.Responses, error) {
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

func (r *LocalJobRunner)run (sql string, pr *PipeReader, pw *PipeWriter) (string, error){
  jobID := jobIDGen()
  r.jobs[jobID] = pr
  go func() {
      defer pw.Close()
      pw.Write(`RUNNING`)
      sqlStatements, _ := sf.SplitMultipleSQL(req.Sql)
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

Upon processing a `Run` request, the server launches a Kubernetes Pod and return the Pod ID and log view URL to the client.
Upon processing a `Fetch` request, the server checks the Pod status and returns the `JobStatus`.

```go
type KubernetesJobRunner {
}

func (r *KubernetesJobRunner)run(sql string, pr *PipeReader, pw *PipeWriter) (string, error){
  podID, err := r.launchK8sPod(sql)
  pw.Write(fmt.Sprintf("Logs viewer URL: %s", r.logsViewerURL(podID)))
  return podID, nil
}

func (r *KubernetesJobRunner) fetch(jobID string) (*pb.Result, error) (
  responses := &pb.Responses{}
  responses.job_status := r.PodStatus(jobID)
  return responsesk, nil
)
```


### Store the Trained Model

For example, a tinny `TRAIN` statement:

``` sql
SELECT ...
TRAIN DNNClassifer
WITH
  ...
COLUMN ...
INTO sqlflow_model
```

This SQL statment would save the model named `sqlflow_model` which contains two parts:

1. The `TRAIN` statement, which would be saved as a `.mod` file.
1. The Model weights, which would be saved as a `.tar.gz` file.

An example of a trained model folder is as follows:

``` text
`-sqlflow_model
  |-sqlflow.gob
  `-sqlflow_model.tar.gz
```

There are two steps to save the trained model:

1. SQLFlow server creates a folder `sqlflow_model` on the distributed file system and saves `sqlflow.gob` in it.
1. The machine learning framework saves the model weights file `sqlflow_model.tar` in the same folder.

### Dealing with failures

Q: What if the client forgets to fetch? For example, a user hits `ctrl+C` in the Jupyter Notebook right after the `client.Run`.

A: The client can specify a timeout T. The server will kill the corresponding job if the client doesn't fetch in the last T seconds.
