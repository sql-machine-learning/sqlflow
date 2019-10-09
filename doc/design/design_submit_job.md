# Submit SQLFLow SQL Asynchronously

## Abstract

In the current system, SQLFlow client connects the SQLFlow server with long links. the SQLFlow client sends
a request which contains the user-typed SQL and hung until the SQLFlow server finishes executing the SQL.
Sometimes, it would cost a long time to execute the SQL and lead to gRPC time out.
The purpose of this proposal is to introduce a way to submit the SQLFlow SQL asynchronously to avoid the
time out issue.

## High-Level Design

For each request, the SQLFlow code generator would generate a submitter program in Python, which submit a Job to the Spark/k8s cluster or launch a process on the host. For the cluster job, submitter program would return an URL which is used to monitor the task status, and the SQLflow server would response the URL to SQLFlow client, and

1. Print the monitor URL on the screen so that users can redirect to the monitor page by clicking the URL.
1. The SQLFlow client would request task monitor URL from the SQLFlow server in a polling manner until the returned task status is **FINISHED** or **FAILED**.

For the local job, we can keep long links for the two main reasons:

1. The local jobs are usually experimental tasks with a small dataset, would not perform a long time.
1. SQLFlow client can print the logs on the screen directly; it's easy to debug the SQLFlow job.

The following section describes more details about this proposal.

## Proposal Details

1. Add a new gRPC interface to request the task status

    ``` protobuf
    service SQLFlow {
      rpc Query(QueryRequest) (stream QueryResponse) // query task status
    }

    message QueryRequest {
      string url = 1;
      Session session = 2;
    }

    message QueryResponse {
      enum TaskStatus {
        PENDING = 1;
        FINISHED = 2;
        FAILED = 3;
      }
      TaskStatus task_status = 1;
      string message = 2;
    }
    ```

1. Parse the task monitor URL and task status in SQLFlow server

    For the difference submitter, the methods of parsing task monitor URL from the submitter program logs and fetch the task status from the monitor URL are different, so we need to implement the two function in the `Submitter` interface:

    ``` go
    type Submitter interface{
      ...
      MonitorURL(taskOutput string) (url string, err error) {}
      TaskStatus(url string) (s TaskStatus, err error) {}
    }
    ```
