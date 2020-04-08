# Logs in SQLFlow

## Motivation

In order to know well about the runtime status of the SQLFlow job, we need to count the number of `TRAIN/PREDICT/EXPLAIN/NORMAL` tasks over a period of time. For example, using ELK Stack for log query and analysis‎. Generally, such statistics components are implemented by logs.

## Logging Libraries

1. Rolling file to limit the log file size, [Lumberjack](https://github.com/natefinch/lumberjack) is the solution.
    ```go
    log.SetOutput(&lumberjack.Logger{
        Filename:   "/path/to/sqlflow.log",
        MaxSize:    50,
        MaxAge:     15,
    })
    ```
1. Structured log messages for the ease of parsing and analysis, [Logrus](https://github.com/sirupsen/logrus) is the solution.
    ```go
    import "github.com/sirupsen/logrus"

    func init() {
      logrus.SetOutput(os.Stdout)
    }

    func main() {
      contextLogger := logrus.WithFields(log.Fields{
        "user": "9527",
      })
      // ...
      contextLogger.Info("TRAIN")
    }
    ```

Combine the two libraries:
```go
import (
	"github.com/sirupsen/logrus"
	"github.com/natefinch/lumberjack"
)

func init() {
  logrus.SetOutput(&lumberjack.Logger{...})
}

func main() {
  // Do your staff and logging
}
```

## Log Formatter

Logs in their raw form are typically a text format with one event per line, i.e. :    
`2020-03-10 10:00:14 level={Level} requestID={RequestID} user={UserID} event={event} msg={Metric, Result or Details}`

RequestID: We use workflow ID as request ID if existed, or UUID instead. With the same RequestID, organizing these logs(events) in order can be reduced to a story;
user: The one who submits the SQL;
event: The process of the task, such as: parsing;
msg: Details.

## Log Categories

1. Statistics log
    Including traffic and performance information.
    1. Count the requests;
    1. Count the number of `TRAIN/PREDICT/EXPLAIN/NORMAL` statements respectively, i.e.:
    `2020-03-10 10:00:14 level=INFO requestID=wf697-s29 user=9527 event=parsing sqlType=TRAIN msg="SELECT * FROM TO TRAIN .."`
    1. Count the workflow steps of each phase, such as `pending/completed/failed/..` in `Fetch()` function.
    1. Log the duration of the completed steps by `time.Now().Second()-wf.CreationTimestamp.Second()` in `Fetch()` function.

    However, if a client doesn't call the `Fetch()`, we've no chance to log the duration and workflow steps of each phase. Instead, we can log such information inside the workflow.
1. Diagnostic log
  All of the error logs.

## Severity Levels
  
Currently, we use Go's standard log package in three ways:
- log.Printf
- log.Errorf
- log.Fatalf

We want to keep the three levels, but maps them to the following three log severity levels provided by Logrus:

- `INFO`
  To highlight the progress of the application at a coarse-grained level.
- `ERROR`
  It designates error events that might still allow the application to continue running.
- `FATAL`
  It records very severe error events that will presumably lead the application to abort.

We need this mapping because Logrus can generate structured messages.
