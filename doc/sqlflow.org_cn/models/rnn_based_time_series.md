# RNNBasedTimeSeriesModel

[RNNBasedTimeSeriesModel](https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/rnn_based_time_series.py) 是基于 RNN 结构实现的神经网络模型，常用于时间序列场景的数据预测。

## 输入数据格式

时间序列场景的数据通常会存储在数据库系统中，例如下面是某 APP 历史流量数据，包括 PV/UV 这两个统计维度，`date_str` 列是日期列。

|date_str| uv| pv|
| -- | -- | -- |
|20200101| 1| 11|
|20200102 | 2 | 12 |
|20200103 | 3 | 13 |
|20200104 | 4 | 14 |
|20200105 | 5 | 15 |
|20200106 | 6 | 16 |
|20200107 | 7 | 17 |

在开始训练/预测之前，按要求需要将上述原始数据处理为按行存储的滑动窗口数据。举例来说，我们希望根据历史 3 天的 PV, UV 数据预测未来 2 天的 PV 数据，预处理后的数据如下所示：

| date_str | uv1 | pv1 | uv2 | pv2 | uv3 | pv3 | target
| -- | -- | -- | -- | -- | -- | -- | -- |
| 20200101 | 1| 11| 2| 12 | 3| 13| 14,15|
| 20200102 | 2| 12| 3| 13 | 4| 14| 15,16|
| 20200103 | 3| 13| 4| 14 | 5| 15| 16,17|

预处理的 SQL 程序见：[常用预处理方法--滑动窗口](#滑动窗口)

## 模型参数

### 业务相关

下列参数和业务场景密切相关，需要密切关注：

1. `model.n_in=N`, 其中 `N >= 1`, 表示使用的 N 天的历史数据.
1. `model.n_out=N`, 其中 `N >= 1`, 表示预测未来 N 天的趋势
1. `model.n_features=N`, 其中 `N>=1`, 表示使用 N 个特征作为训练数据，在[上述](#输入数据格式)例子中，因为使用了 PV，UV 两个特征，所以此值为 2。

### 效果相关

下列参数在调优效果时，一般会用到：

1. `model.stack_units=[128,256]`, 神经网络中隐层神经元的数量，默认值`[128, 256]`, 通常增加神经元数量会使效果有所提升，同时训练速度降低。
1. `train.epochs=N`, 其中 `N >= 1`, 表示完整的数据集在神经网络中传递 N 次, 默认为 `10`。可以观察每个 epoch 的训练效果，如果跨度较大，可以调大此参数增加数据在神经网络中传递的次数，直至效果收敛。
1. `train.batch_size=N`, 其中 `N >= 1`, 默认值为 `8`。完整的数据集如果不能一次性的通过整个网络时，需要将其切成几个小的 batch，每个 batch 包含 N 个样本。

### 模型验证

1. `train.validation='SELECT ...'`, 通过 `SELECT` 子句选取验证集，注意和训练集的列名，字段类型要保持一致。
1. `train.metrics='MeanAbsoluteError, MeanAbsolutePercentageError, MeanSquaredError'`, 表示验证模型时使用的口径,其中：
    - `MeanAbsoluteError` 表示平均绝对误差。
    - `MeanAbsolutePercentageError` 表示平均绝对误差的百分比。
    - `MeanSquaredError` 表示平均相对误差。

## 常用的数据预处理方法

### 滑动窗口

``` sql
CREATE TABLE tmp_train AS
  SELECT
    date_str,
    uv AS uv1,
    pv AS pv1,
    LEAD(uv, 1) OVER (ORDER BY date_str) AS uv2,
    LEAD(pv, 1) OVER (ORDER BY date_str) AS pv2,
    LEAD(uv, 2) OVER (ORDER BY date_str) AS uv3,
    LEAD(pv, 2) OVER (ORDER BY date_str) AS pv3,
    CONCAT_WS(",",
            LEAD(pv, 3) OVER (ORDER BY date_str),
            LEAD(pv, 4) OVER (ORDER BY date_str)) AS target
  FROM app_traffic;
```

## 应用案例

- [GEFCom2014 Energy Forecasting](/doc/tutorial/energe_lstmbasedtimeseries.md)
