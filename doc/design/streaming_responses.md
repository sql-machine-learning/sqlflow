# 支持流式响应

用户在提交任务之后，希望能实时查看任务执行状态。所以，sqlflow需要实现流式响应。

## 响应消息的内容

如果任务是standard SQL，sqlflow 按对应 SQL 引擎返回，不作改动；    
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

这无法满足流式需求。Go语言中，流一般是通过goroutine和channel来实现的。我们可以将function signature改成

```go
// Response for Run return, contains: 
// - data: string or database row 
type Response struct {
    data interface{}
    err  error
}

func Run(slct string, ...) chan Response {}
func runStandardSQL(slct string, ...) chan Response {}
func runExtendedSQL(slct string, ...) chan Response {}
```

这样在sqlflowserver，只需要

```go
package sqlflowserver

import "sqlflow"

// ...
rspChan := sqlflow.Run(slct, db, testCfg)
for rsp := range rspChan {
    stream.Send(&RunResponse{rsp})
}       
```

### 实现
```go
package sqlflow

type logChanWriter struct {
    c chan Response
    buf bytes.Buffer
    // ...
}

func runExtendedSQL(slct string, ...) chan Response {
    rspChan := make(chan Response)
    go func() {
        defer close(rspChan)
        // collect response from train or infer
        // Parse
        // Open database
        // Create Temp dir
    }()
    return rspChan
}

func train(...) chan Response {
    c := make(chan Response)
    go func() { 
        defer close(c)
        err := func() error {
            // ...
            cw := &logChanWriter{c: c}
            cmd := tensorflowCmd(cwd)
            cmd.Stdin = &program
            cmd.Stdout = cw
            cmd.Stderr = cw
            // ...
        }
        if err != nil {
            c <- Response{"", err}
        }
    }
    return c
}
```

## 涉及改造的点
按重要程度排列，

1. sqlflow 与 sqlflowserver 集成（以 pysqlflow 为客户端做测试），需要：  
    1. sqlflowserver 从 channel 中读取信息  
    1. 按流程，sqlflow 构造写入 channel 的信息。也包括error **done**
1. 生成tensorflow的python代码，重定向其输出: 实现方式不确定，打算先做示例跑通。需要考虑当channel中的对象是FlowLog时    
**done**
1. Standard SQL 的返回结果并非总是table，在构造返回消息体时，如何判断消息类型？  
**done**
1. 如何判断table中row的类型？初步想法：可以通过empty interface作为返回值。然后select type来做
**done**
1. 异常信息返回给用户端，是否需要做区分？即 [A gRPC server should be able to return errors to the client](https://github.com/wangkuiyi/sqlflowserver/issues/19)
1. 控制单条消息的 max size：只要控制返回的 table 大小即可。简单地可通过 limit 约束。
1. timeout处理
