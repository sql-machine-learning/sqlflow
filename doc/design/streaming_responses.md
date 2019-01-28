# 支持流式响应

用户在client端提交任务之后，希望能实时查看任务执行状态。所以，server端需要实现流式响应。

## 响应消息的内容

如果任务是standard SQL，server端按对应SQL引擎返回table，不作改动。

如果任务是extended SQL，server端会依次返回如下信息：

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

原sqlflow的执行函数`run()`将结果一次性返回，无法满足流式需求。因此需要一种机制能在`run()`之外获取到`run()`之内的信息。按[Tony的建议](https://github.com/wangkuiyi/sqlflowserver/issues/18#issuecomment-457790587)，这里使用[channel](https://tour.golang.org/concurrency/2)为通信载体。

###  extended SQL
- sqlflowserver   
```go
func runExtendedSQL(slct, stream) {
  logChan := make(chan FlowLog)
  go sqlflow.runExtendedSQL(slct, logChan)
  for log := range logChan {
    response := &RunResponse {
      // TODO: log response
    }
    stream.Send(rsp)
  }
}
```

- sqlflow
```go
func train(..., logChan chan FlowLog) error {
  fts, e := verify(tr, db)
  logChan <- &FlowLog{msg: "verify done"}
  
  var program bytes.Buffer
  if e := genTF(&program, tr, fts, cfg); e != nil {
    return e
  }
  logChan <- &FlowLog{msg: "codegen done"}
  cmd := tensorflowCmd(cwd)
  cmd.Stdin = &program
  // TODO: redirect output to logChan
  cmd.Stdout = logChan
  o, e := cmd.CombinedOutput()
  if e != nil || !strings.Contains(string(o), "Done training") {
    return fmt.Errorf("Training failed %v: \n%s", e, o)
  }
  
  logChan <- &FlowLog{msg: "model save done"}
  m := model{workDir: cwd, TrainSelect: slct}
  return m.save(db, tr.save)
}
```

Q: 为什么需要 FlowLog，而不是 string?    
`表达 stdout & stderr`

### standard SQL
*TODO*

## 涉及改造的点
按重要程度排列，

1. 生成tensorflow的python代码，重定向其输出。   
`实现方式不确定，打算先做示例跑通。需要考虑当 channel 中的对象是 FlowLog 时`
2. sqlflow 与 sqlflowserver 集成（以 pysqlflow 为客户端做测试），需要：  
2.1. sqlflowserver 从 channel 中读取信息  
2.1. 按流程，sqlflow 构造写入 channel 的信息。也包括异常

3. standard SQL 的结果并非总是table，在构造返回消息体时，如何判断消息类型？    
`存疑`
4. 异常信息返回给用户端，是否需要做区分？即 [A gRPC server should be able to return errors to the client](https://github.com/wangkuiyi/sqlflowserver/issues/19)
5. 控制单条消息的 max size  
`只要控制返回的 table 大小即可。简单地可通过 limit 约束`
