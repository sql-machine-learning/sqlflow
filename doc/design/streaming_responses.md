# 支持流式响应

用户在提交任务之后，希望能实时查看任务执行状态。所以，sqlflow需要实现流式响应。

## 响应消息的内容

如果任务是standard SQL，sqlflow按对应SQL引擎返回table，不作改动。

如果任务是extended SQL，sqlflow会依次返回如下信息：

1. 任务的准备：
    1. `Done pasrsing`
    1. `Done verifying`
    1. `Done code generation`
    1. `...`
1. 任务的执行：
    1. `Start training`
    1. `epoch 0, train_loss = ...`
    1. `epoch 1, train_loss = ...`
    1. `...`
    1. `Done training`
    1. `Saving model into ...`
1. 任务的结束
    1. `Job finished. Time elapsed ...`

## 如何支持流式

### Function signature

目前sqlflow的`runStandardSQL`和`runExtendedSQL`都会将结果一次性返回

```go
func runStandardSQL(slct string, ...) (string, error) {}

func runExtendedSQL(slct string, ...) (string, error) {}
```

这无法满足流式需求。在Golang，流一般是通过goroutine和channel来实现的。我们可以将function signature改成

```go
struct Row {
    Row []interface{}
}

struct Log {
    log string
}

func runStandardSQL(slct string, ...) chan Row {}

func runExtendedSQL(slct string, ...) chan Log {}
```

这样在sqlflowserver，只需要

```go
package sqlflowserver

import "sqlflow"

func runExtendedSQL(slct, stream) error {
    logChan := sqlflow.runExtendedSQL(slct)
    for log := range logChan {
        stream.Send(&RunResponse{log})
    }
}
```

### 实现

```go
package sqlflow

func runExtendedSQL(slct string, ...) chan Log {
    chanLog := make(chan Log)
    go func() {
        // Parse
        // Open database
        // Create Temp dir
        if pr.train {
            train(..., logChan chan Log)
        } else {
            infer(..., logChan chan Log)
        }
    }()
    return chanLog
}

func train(..., logChan chan Log) {
  fts, e := verify(tr, db)
  logChan <- &Log{log: "verify done"}
  
  var program bytes.Buffer
  if e := genTF(&program, tr, fts, cfg); e != nil {
    return e
  }
  logChan <- &Log{log: "codegen done"}
  cmd := tensorflowCmd(cwd)
  cmd.Stdin = &program
  // TODO: redirect output to logChan
  cmd.Stdout = logChan
  o, e := cmd.CombinedOutput()
  if e != nil || !strings.Contains(string(o), "Done training") {
    return fmt.Errorf("Training failed %v: \n%s", e, o)
  }
  
  logChan <- &Log{log: "model save done"}
  m := model{workDir: cwd, TrainSelect: slct}
  return m.save(db, tr.save)
}
```

Q: 为什么需要 FlowLog，而不是 string?    
`表达 stdout & stderr`

### standard SQL
*TODO*

## 涉及改造的点
按重要程度排列，

1. sqlflow 与 sqlflowserver 集成（以 pysqlflow 为客户端做测试），需要：  
    1. sqlflowserver 从 channel 中读取信息  
    1. 按流程，sqlflow 构造写入 channel 的信息。也包括error
1. 生成tensorflow的python代码，重定向其输出: 实现方式不确定，打算先做示例跑通。需要考虑当channel中的对象是FlowLog时
1. Standard SQL 的返回结果并非总是table，在构造返回消息体时，如何判断消息类型？
1. 如何判断table中row的类型？初步想法：可以通过empty interface作为返回值。然后select type来做
1. 异常信息返回给用户端，是否需要做区分？即 [A gRPC server should be able to return errors to the client](https://github.com/wangkuiyi/sqlflowserver/issues/19)
1. 控制单条消息的 max size：只要控制返回的 table 大小即可。简单地可通过 limit 约束。
