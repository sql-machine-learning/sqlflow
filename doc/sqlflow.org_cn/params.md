# 参数手册

# 模型训练参数配置

模型训练参数是指SQLFlow SQL语句中`WITH`后可以添加的指定模型训练细节的值，使用方法为：

```sql
SELECT ...
TO TRAIN [YourModel]
WITH param=value, param=value, ...
LABEL ...
INTO ...
```

## 模型参数

执行模型的配置需要以`model.`开头，比如`model.n_classes=3`。不同的模型会有不同的参数配置。详细参考：[模型手册](models.md)。

## 训练过程参数

| 参数 | 类型 | 默认值 | 说明 |
| -------- | -------- | -------- | -------- |
| train.batch_size  |  int | 1 | 训练的batch size，int类型，如：256 |
| train.epoch | int | 1 | 训练的轮数，int类型，如：10 |
| train.verbose | int | 0 | 训练日志Level，可以是0,1,2，2为最详细 |
| train.max_steps | int | 0 |最大训练步数。如果指定此项，train.epoch指定的轮数可能不生效，只按照最大步数停止训练，0表示训练所有数据 |
| train.save_checkpoints_steps ｜ int | 100 | 保存checkpoint经过的步数 |
| train.log_every_n_iter | int | 10 ｜ 打印日志的步数间隔 |

## 评估过程参数

| 参数 | 类型 | 默认值 | 说明 |
| -------- | -------- | -------- | -------- |
| validation.select  | string | "" |  获取validation数据集的SQL语句 |
| validation.start_delay_secs | int | 0 | 开始第一次执行validation等待的时间（秒），0表示不等待 |
| validation.throttle_secs | int | 0 | 第二次执行validation之前需等待的时间（秒），0表示不等待 |
| validation.metrics | string | "Accuracy" |（Keras模型适用）需要输出的模型评估指标，默认"Accuracy"，需要使用多个指标则使用逗号分割，如："Accuracy,AUC"，支持的指标包括：Accuracy, Precision, Recall, AUC, TruePositives, TrueNegatives, FalsePositives, FalseNegatives, BinaryAccuracy, CategoricalAccuracy, TopKCategoricalAccuracy, MeanAbsoluteError, MeanAbsolutePercentageError, MeanSquaredError, RootMeanSquaredError |
| validation.steps | int | 0 | 执行validation的步数，0表示训练所有数据 |

## Optimizer配置方法

对于Estimator模型和Keras模型都可以用下面方法配置训练的优化器 (Optimizer)：

```sql
WITH model.optimizer=AdagradOptimizer,
     optimizer.learning_rate=0.001,
     optimizer.initial_accumulator_value=0.1
```

1. `model.optimizer` 指定使用的optimzer
2. `optimizer.*`配置该optimizer的初始化参数。

可以支持的optimizer和对应的参数详情可以对应参考https://www.tensorflow.org/api_docs/python/tf/keras/optimizers。

# 模型解释参数配置

模型解释参数是指 SQLFlow 模型解释 SQL 语句中 `WITH` 后可以添加的细节的配置，使用方法为：

```sql
SELECT ...
TO EXPLAIN [YourModel]
[USING Explainer]
WITH param=value, param=value, ...
[INTO ...];
```

模型解释只支持一个可配置参数：`summary.plot_type`，用于配置输出的模型解释的结果图表。可选的值包括：

- `bar`：柱状图
- `dot`：点阵图
- `decision`：决策图

# 模型评估参数配置

模型评估参数是指 SQLFlow 模型评估 SQL 语句中 `WITH` 后可以添加的细节配置，使用方法为：

```sql
SELECT ...
TO EVALUATE [YourModel]
WITH param=value, param=value, ...
LABEL ...
[INTO ...];
```

| 参数 | 类型 | 默认值 | 说明 |
| -------- | -------- | -------- | -------- |
| validation.metrics | string | "Accuracy" |（Keras模型适用）需要输出的模型评估指标，默认"Accuracy"，需要使用多个指标则使用逗号分割，如："Accuracy,AUC"，支持的指标包括：Accuracy, Precision, Recall, AUC, TruePositives, TrueNegatives, FalsePositives, FalseNegatives, BinaryAccuracy, CategoricalAccuracy, TopKCategoricalAccuracy, MeanAbsoluteError, MeanAbsolutePercentageError, MeanSquaredError, RootMeanSquaredError |