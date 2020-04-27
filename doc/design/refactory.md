# 代码重构随想

# 一、主要参考资料
## SQLFlow重构初步规划
这部分试将[原文](https://github.com/sql-machine-learning/sqlflow/issues/1434#issuecomment-575966175)的主要内容总结如下。


### 目录组织
顶层package分为几部分: 
- parser
- ir
- resolver
- verifier
- codegen
- executor

一条语句过来，按顺序从上至下经过这四个目录下的模块。


### 流程组织
从执行流程来看，每个package提供一个到两个主要的函数：

- parser.Parse
- resolver.ResolveProgram
- verifier.Verify
- workflow.GenCode
- workflow.Run

一条语句过来，按顺序从上到下执行一系列函数调用
### 未解决问题
以上讨论集中在big picture，一些更细节的内容尚未涉及

1. 尚未涉及sqlflow_submitter这个Python package
    - 此package和Python API的关系？
1. 尚未涉及如何使支持新的引擎(如PyTorch)、新的平台(如PAI)更加自然
    - 目前PAI部分的codegen和TensorFlow的codegen之间的关系较为复杂
1. 尚未涉及diagnostics在系统中的位置
    - attribute check作为resolver/verifier的一部分？
    - 仿照其它语言的异常机制，设计一个Error体系？
    - Python部分的运行时错误如何在Golang中更好地报告
1. 尚未涉及如何定义sqlflow_models和Model Zoo的规范，使用户自定义模型能更好融入到原有框架中


## MySQL源码组织

作为老牌开源DBMS，有必要看看MySQL是否有我们值得借鉴之处，特别是在数据库概念的组织上

MySQL提供了一份[源码阅读指南](https://dev.mysql.com/doc/internals/en/sql-directory.html)。有趣的是，MySQL和SQLFlow一样，也有个sql目录，看来由于出现过早，有些问题并未妥善解决。

大部分流程MySQL都放在了[sql/sql_parse.cc](https://github.com/mysql/mysql-server/blob/8.0/sql/sql_parse.cc)这个文件中，该文件约有7000行，这个文件parse SQL后，做一些基本检查，然后分发到其余文件，如SELECT语句会调用[sql/sql_select.cc](https://github.com/mysql/mysql-server/blob/8.0/sql/sql_select.cc)，SHOW语句会分发给sql/sql_show.cc等。这样看起来也比较清晰，后续我们如果需要支持了更多语法的话，也可以参考这个做法。

MySQL的诊断信息并未独立成模块，而是分散在各处，通过[my_error函数](https://github.com/mysql/mysql-server/blob/91a17cedb1ee880fe7915fb14cfd74c04e8d6588/mysys/my_error.cc)作为报错的统一出口，同时错误会归类到mysqld_error.h等头文件中。也有部分是用[C++ exception解决](https://github.com/mysql/mysql-server/blob/4869291f7ee258e136ef03f5a50135fe7329ffb9/sql/sql_exception_handler.cc)。总的来说这部分值得借鉴的主要是MySQL为各种error归类编号的方式。

_TODO: 由于MySQL代码较多，并未深入其细节。后续补充必要内容。_


## clang源码组织
clang作为编译器中的后起之秀，代码组织相对于MySQL清晰了很多，是个[基于lib的架构](https://github.com/llvm-mirror/clang/tree/master/lib)，这和我们初步规划的内容较为接近。

clang的核心是按lib来组织各个模块，和SQLFlow类似的目录包括
- Lex
- Parse
- AST
- Sema
- CodeGen

值得称道的是，clang虽然没有把Diagnostics放到一个单独的lib中，但在Basic这个lib下有非常多以diagnostics命名的代码文件。可见其将这部分当作一个重要的事情来设计和实施，clang刚出来的时候能从gcc分到一杯羹也有一个重要的原因是诊断信息更加易读。

当然，SQL语言本身比C++简单得多，我们不需要做出如此多的工作，但确实可以借鉴clang，将所有诊断信息都汇总起来。另外，clang将语义检查放到Sema目录下，作为一个重要阶段。这也体现了其对诊断的重视程度，因为语义检查和错误诊断信息二者在大部分语言中都是息息相关的。

_TODO: 由于clang代码较多，并未深入其细节。后续补充必要内容。_


# 二、后续方向设想
这部分是接下来工作初步的设想，还缺少很多细节，主要来自直觉和本文第一部分，接下来一段时间会不断深化思考，进行调整，最终的方案和当下版本可能会有区别。

## 语义检查
将第一部分提及的Resolve、Verify，和第一部分未提及的codegen/attribute/都放到一个顶级目录下，参考clang，取名叫sema，这部分的目标是将尽量多的检查前置，使问题在运行的早期尽快暴露出来。

- 这部分工作量会集中在"在Golang中为sqlflow_submitter中的Python代码增加语义检查"上
- 这部分工作虽然叫sema，但并非只是static type checking，而是包含可前置检查的所有语义
- 目标是让尽量多的SQL程序中的问题能在用户敲下回车/点击运行之后一秒内发现，再通过diagnostics模块报告出来


## 诊断系统diagnostics
在顶级目录下新增这一目录，参考clang和MySQL，将所有可能出现的错误统一整理，提供统一出口，diagnostics的作用更多是作为lib供其他所有模块(如sema)调用。

- 这部分工作需要整理目前所有可以前置检查的错误类型(如语义检查)，规避"ERROR: runSQLProgram error: unexpected type on attribute train.epoch. expect int, received a(string)"这样的报错方式。统一为：“Error 1046: unexpected type on attribute 'train.epoch': expect int, received 'a'(string)”等。
- 这部分工作需要解决Python报出运行时错误的问题，不能再打印出一大串Python源码来惊吓用户: 目前如果输入
    ```sql
    SELECT * FROM iris.train TO TRAIN DNNClassifier WITH model.hidden_units=123 LABEL class INTO gogogo;
    ```
    用户将看到约200行的错误提示，错误提示的末尾是"TypeError: 'int' object is not iterable"，对用户也没什么提示作用。当然，这个问题需要通过上文提到的"在Golang中为sqlflow_submitter中的Python代码增加语义检查"来解决，但也可能存在无法在语义检查阶段处理的运行时错误，如果有这种情况，我们只告诉用户“Error 1046: training failed: TypeError: 'int' object is not iterable”即可
- 这部分工作需要提供错误列表手册，用户在遇到棘手问题时可以通过查询编号自行解决
- 这部分工作需要提供解决我们自身debug的问题，错误信息对应的stacktrace应当记录在后台日志系统，方便定位
- 这部分工作需要汇总PAI、DataWorks、MySQL、MaxCompute等各种日志和错误，归纳分类


## codegen
该目录需要
1. 理清PAI TF和TensorFlow的关系，予以更合理的组织方式
    - 同时，理清「平台」和「引擎」之间的关系
2. COLUMN子句从TensorFlow codegen中移出，理清COLUMN子句的定位
2. 提供统一的模型存储格式(nice to have)
    - PMML、SavedModel、RTP...
4. 抽象添加新引擎所需工作，提供相应指导文档
    - 参考Golang image模块，采用注册机制？
    - 对引擎予以合理的封装，对上提供统一的接口，例如：
    ```go
    type Function struct {
        Run()  // SELECT ... TO RUN ...
    }
    type Engine interface {
        SemaCheck()  // Call package `sema', called by Train/Predict/Explain/Evaluate
        Transform()
        Train()
        Predict()
        Explain()
        Evaluate()
    }
    ```
	而不是像现在这样，每个引擎都在package下放置Train、Predict等函数，函数签名也都不一致，新增引擎往往需要改动很多文件中的多处代码。最理想的状态是，新增引擎/平台只需要新增文件，而不需要动已有的任何文件


## sqlflow_submitter
该目录可认为是SQLFlow这门「编程语言」的runtime，如libstdc++.so之于gcc，需要在重构工作中精心重铸：

1. 理清和codegen的关系
    - 将golang codegen中可能移入的代码尽量移入？
2. 提供统一接口，如：
```python
# 如果此处能封装好，则golang的codegen就有望不需要再封装，所需工作将大大减轻
# 代码结构也会更清晰
class Engine():
    @abstractmethod
    def Transform():
    	pass
    @abstractmethod
    def Train():
        pass
    @abstractmethod
    def Predict():
        pass
    @abstractmethod
    def Explain():
        pass
    @abstractmethod
    def Evaluate():
        pass
```

3. 以sqlflow_submitter为基础，提供Python API（倘若如此，sqlflow_submitter可改名为sqlflow了？）
    - 从以下代码可以看出，对xgboost和tensorflow而言，通过Python调用sqlflow_submitter的语法和我们之前讨论过的sqlflow Python API的写法颇为类似：
    ```python
    # pkg/sql/codegen/xgboost/template_train.go
    train(datasource='''{{.DataSource}}''',
          select='''{{.TrainSelect}}''',
          model_params=model_params,
          train_params=train_params,
          feature_metas=feature_metas,
          feature_column_names=feature_column_names,
          label_meta=label_meta,
          validation_select='''{{.ValidationSelect}}''',
          disk_cache="{{.DiskCache}}" == "true",
          batch_size={{.BatchSize}},
          epoch={{.Epoch}},
          is_pai="{{.IsPAI}}" == "true",
          pai_train_table="{{.PAITrainTable}}",
          pai_validate_table="{{.PAIValidateTable}}")
    
    # pkg/sql/codegen/tensorflow/template_train.go
    train(datasource="{{.DataSource}}",
          estimator={{.Estimator}},
          select="""{{.TrainSelect}}""",
          validation_select="""{{.ValidationSelect}}""",
          feature_columns=feature_columns,
          feature_column_names=feature_column_names,
          feature_metas=feature_metas,
          label_meta=label_meta,
          model_params=model_params_constructed,
          validation_metrics="{{index .ValidationParams "metrics"}}".split(","),
          save="{{.Save}}",
          batch_size={{index .TrainParams "batch_size" | attrToPythonValue}},
          epoch={{index .TrainParams "epoch" | attrToPythonValue}},
          validation_steps={{index .ValidationParams "steps" | attrToPythonValue}},
          verbose={{index .TrainParams "verbose" | attrToPythonValue}},
          max_steps=train_max_steps,
          validation_start_delay_secs={{index .ValidationParams "start_delay_secs" | attrToPytho
    nValue}},
          validation_throttle_secs={{index .ValidationParams "throttle_secs" | attrToPythonValue
    }},
          save_checkpoints_steps={{index .TrainParams "save_checkpoints_steps" | attrToPythonVal
    ue}},
          log_every_n_iter={{index .TrainParams "log_every_n_iter" | attrToPythonValue}},
          is_pai="{{.IsPAI}}" == "true",
          pai_table="{{.PAITrainTable}}",
          pai_val_table="{{.PAIValidateTable}}")
    ```
	理想的状态应该是，sqlflow_submitter升级为SQLFlow Python API，golang的codegen所需要做的唯二两件事是: a)类型检查和b)转发参数给Python API。这样一来，新增引擎的工作通过在Python中新增文件实现，golang的codegen不需要任何改动或只需要新增极少代码(如调用sema配置类型检查)，这样得到的Python API也更为自然


## server和proto
server和proto都是pkg下现存的目录之一，这块在设想中唯一需要做的工作是将HDFS相关的session移出：
```protobuf
message Session {
    string token = 1;
    string db_conn_str = 2;
    bool exit_on_submit = 3;
    string user_id = 4;
    // for loading CSV to hive
    string hive_location = 5;
    string hdfs_namenode_addr = 6;
    string hdfs_user = 7;
    string hdfs_pass = 8;
    string submitter = 9;
}
```
不管怎么说，HDFS和Hive相关的配置可能确实需要放到一个更合适的位置。


## 小结
设想中pkg目录的组织：

- parser
- ir
- sema
  - resolver
  - verifier
  - attribute
- diagnostics
- codegen

_TODO: 这部分写得很简略，待其余部分完善后补充细节。_
