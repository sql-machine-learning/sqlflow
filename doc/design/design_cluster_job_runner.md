# SQLFlow Cluster Job Runner

## Motivations

In the current system, SQLFlow client connects the SQLFlow server with a long live connection. the SQLFlow client sends
a gRPC request which contains a SQL statement and blocking until the SQLFlow server finishes executing the SQL statement.

For each SQL statement, the SQLFlow code generator would generate a submitter program in Python, and then the SQLFlow server
would launch a process on the host or launch a distributed Job on cluster(Kubernetes/Yarn), this would cause two problems in the production environment:

1. The local job may cause the SQLFlow server resource insufficient when there are too much SQL jobs.
1. Sometimes, the SQL job takes too much time and the gRPC calls timeout.
1. The SQLFlow server is not High-Available, if an SQLFlow server instance failed, the jobs on this instance are failed.

In this design, we propose to implement the **Cluster Job Runner** to solve the above problems.

The cluster job runner would launch the job on Kubernetes or Yarn cluster instead of on the host. The SQLflow client can
check the job status in a polling manner instead of a long live connection. We recommend using the cluster job runner in the production environment.

## High-Level Design

For the most submitter program, it can run as the local mode or distributed mode. For the local mode, SQLFlow would run
the submitter program as a local process on the host; For the distributed model, the submitter program would submit a distributed Job to the cluster. For the two modes, the behavior of SQLFlow is different in both local job runner and cluster job runner:

Job Runner| local mode | distributed mode
-- | -- | --
Local | launch a process on the host with blocking| submite a job to cluster with blocking
Cluster| launch a Kubernetes Pod with no-blocing| submite a job to cluster with no-blocking

The cluster job runner workflow is as follows:

<img src="figures/cluster_job_runner.png">

1. SQLFlow client sends the SQL statement via a gRPC call to the SQLFlow server. The local job goto **2** and the distributed
job go to **3**.
2. If the SQL statement implies a local job, e.g. local TensorFlow Job and local XGBoost job.
    1. SQLFLow server would launch a Pod on Kubernetes cluster via Kubernetes API.
    1. SQLFLow server can fetch the job tracker URL from the api-server.
    1. The SQLFlow server returned the job tracker URL to the client.
3. If the SQL statement implies a distributed job, e.g. distributed Tensorflow Job, distributed ALPS job, or other distributed
machine learning job.
    1. SQLFlow server would call the distributed AI framework API to launch a distributed Job on a cluster(Kubernetes/Yarn),
    e.g. EDL API or ALPS API.
    1. SQLFLow server can fetch the Job tracer URL from the distributed AI framework API.
    1. SQLFlow server returns the job tracer URL to the client.
4. The SQLFLow client would send the job tracer URL to the SQLFlow server to request the task status in a polling manner until
the returned task status is **COMPLETED** or **FAILED**.

## Proposal Details

1. Add a new gRPC interface to check the job status.

    ``` protobuf
    service SQLFlow {
      /**
      if sqlflow_job_runner == "local":
          sqlflow.Run(sql...)
      elif sqlflow_job_runner == "cluster":
          res = sqlflow.Run(sql...)
          is_finised = False
          while !is_finished:
              is_finised = sqlflow.IsFinished(res.session.job_tracker_url)
      **/
      rpc IsFinished(Request) (stream TaskStatus) // add a new gRPC interface to request the job status
      rpc Run (Request) returns (stream Response);
    }

    message Session {
      ...
      string job_tracer_url = 5; // add a new filed `url` in Session
    }

    message Response {
        oneof response {
            ...
            Session session = 5; // add session in Response
        }
    }

    message TaskStatus{
      enum Code {
        PENDING = 0;
        COMPLETED = 1;
        FAILED = 2;
      }
      string message = 0;
    }
    ```

1. Parse the job tracker URL and task status in SQLFlow server

    We can implement two functions `jobTrackerURL` and `jobStatus` to parse the task tracker URL and task status:

    ``` go
    type Submitter interface{
      ...
      jobTrackerURL(taskOutput string) (url string, err error) {}
      jobStatus(url string) (s TaskStatus, err error) {}
    }
    ```

1. Store the trained-model

    For a `TRAIN` statment:

    ``` sql
    SELECT ...
    TRAIN DNNClassifer
    WITH
        ...
    COLUMN ...
    INTO sqlflow_model
    ```

    The SQLFlow trained-model contains two parts, one is model weights and the other a serialization file of SQLFlow
    Model Struct which includes the column spec in the **TRAIN** statement, an example trained model files is as follows:

    ``` text
    `-sqlflow_model
      |-sqlflow.gob
      `-sqlflow_model.tar.gz
    ```

    There are two steps to save the trained model:
    1. SQLFLow server creates a folder `sqlflow_model` on the distributed file system and saves `sqlflow.gob` in it.
    1. The machine learning framework saves the model weights file `sqlflow_model.tar` in the same folder.

## Alternative high-level design:

### Client

The client calls `Run` and receives a string token. The client subsequently fetches the result using the token periodically. And the client is unaware of the deployment type of the server.

```python
token = client.Run("sqls ...")
while True:
    result = client.Fetch(token)
    for r in result.results:
        # do stuff: either print logs or construct Rows
    if result.job_status in (SUCCEEDED, FAILED):
        break
    sleep(some_time)
```

And the Protocol Buffer definition looks like the following

```proto
service SQLFlow {
    // Client calls Run and receives a string token.
    rpc Run (Request) returns (Token);
    // Client subsequently fetches the result using the token periodically.
    rpc Fetch(Token) returns (Result);
}

message Token {
    string token = 1;
}

message Session {
    string token = 1;
    string db_conn_str = 2;
    bool exit_on_submit = 3;
    string user_id = 4;
}

message Result {
    JobStatus job_status = 1;
    // Fetches multiple responses at once
    repeated Response results = 2;
}

message JobStatus {
    enum Code {
        PENDING = 0;
        RUNNING = 1;
        SUCCEEDED = 2;
        FAILED = 3;
        UNKNOWN = 4;
    }
}
```

### Server

Upon processing a `Run` request, the server generates, bookkeeps and returns the token to the client. Upon processing a `Fetch` request, the server looks up the result channel and returns the most recent result.

```go
func (s *Server) Run(ctx context.Context, req *pb.Request) (*pb.Token, error) {
   db := s.db
   sqlStatements, _ := sf.SplitMultipleSQL(req.Sql)

   token := tokenGen()
   pr, pw := sf.Pipe()
   s.jobs[token] = pr

   go func() {
      defer pw.Close()
      pw.Write(`RUNNING`)
      for _, singleSQL := range sqlStatements {
         for e := range s.run(singleSQL, db, s.modelDir, req.Session).ReadAll() {
            pw.Write(e)
         }
      }
      pw.Write(`SUCCEEDED`)
   }()

   return &pb.Token{Token: token}, nil
}

func (s *Server) Fetch(ctx context.Context, token *pb.Token) (*pb.Result, error) {
   result := &pb.Result{}
   pr, ok := s.jobs[token.Token]
   if !ok {
      return nil, fmt.Errorf("unrecognized token %s", token.Token)
   }
   for ;; {
      select {
      case res := <-pr.ReadAll():
         // construct result
      case <-time.After(1 * time.Second):
         break;
      }
   }
   return result, nil
}
```

Since we have multiple gRPC calls for the same SQLFlow job, we need to maintain a state across different calls. We certainly can't store the state on the client since the client doesn't know the number of SQLs/tasks. So we decide to save the state as a token&`PipeReader` pair.

### Third-party computation service

The server calls the third party computation service synchronously.

### Dealing with failures:

Q: What if the client forgets to fetch? For example, a user hits `ctrl+C` in the Jupyter Notebook right after the `client.Run`.

A: The client can specify a timeout T. The server will kill the corresponding job if the client doesn't fetch in the last T seconds.

