# Logs in SQLFlow
## Motivation
In order to know well about the runtime status of the SQLFlow job, we need to count the number of `TRAIN/PREDICT/EXPLAIN/NORMAL` tasks over a period of time. For example, using ELK Stack for log query and analysis‎. Generally, such statistics components are implemented by logs.

## logging library
Several statistics components use `logtail` to collect logs. The `logtail` demands a file. If such information is written to `stdout`, we should redirect the `stdout` to a local file, the file size will increase until the SQLFlow server terminates, so we need a rolling file to store the log. Besides, a well-formatted log is readable.
We use [Logrus](https://github.com/sirupsen/logrus) for layout, combining with [https://github.com/natefinch/lumberjack] for rolling the log file.

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
    1. Log the duration of the completed steps by `time.Since(pod.StartTime)` in `Fetch()` function.

    However, if a client doesn't call the `Fetch()`, we've no chance to log the duration and workflow steps of each phase. Instead, we can log such information inside the workflow.
1. Diagnostic log
  All of the error logs.

## Log Level
We use:
1. `INFO` log used to highlight the progress of the application at a coarse-grained level, so the statistics information recorded by the `INFO` log.
1. `ERROR` log designates error events that might still allow the application to continue running.
1. `FATAL` log records very severe error events that will presumably lead the application to abort.