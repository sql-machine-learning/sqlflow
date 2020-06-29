# SQLFlow 模型列表

SQLFlow 模型库(Model Zoo) 中提供了很多算法、模型，您可以根据要解决问题的不同，选择其中一个在现有的数据集上进行尝试，
同时每个模型我们也提供了相应的业务案例供您参考。

## 选择模型

1. [探索数据规律](#探索数据规律)，适用于发现数据集中数据集上发现一些规律特征，解决 “数据中有什么” 的问题。
1. [数值预测](#数值预测)，适用于数据中已有一些关联关系，根据这些关系在一个新的数据集上预测，解决 “有多少” 的问题。
1. [分类预测](#分类预测)，数据集中存在两个或多个类别，推测新的数据是属于哪一类，解决“是或否“(二分类)， “A 或 B 还是 C”（多分类）的问题。

### 探索数据规律

1. [Deep Embedding Clustering](/doc/sqlflow.org_cn/models/deep_embedding_clustering.md) 无监督聚类算法，
将拥有相似特点的数据，划分到同一个组中 。

### 数值预测

1. [DNNRegression](https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/dnnregressor.py) 基于神经网络
    的回归模型，可以预测单一数值。
1. [RNNBasedTimeSeriesModel](/doc/sqlflow.org_cn/models/rnn_based_time_series.md) 基于过去一段时间的数据，预测未来一段时间内的趋势，用来预测一组基于时间序列的数值。
1. [XGBoost Regression](#) 基于树模型的的回归模型。

| 模型| 训练速度 | 图形化模型解释 | 是否支持大规模数据 |
| -- | -- | -- | -- |
| DNNRegresssion | 慢 | 支持 | 支持 |
| [RNNBasedTimeSeriesModel](/doc/sqlflow.org_cn/models/rnn_based_time_series.md)| 慢 | 不支持 | 不支持 |
| XGBoost Regression | 快 |支持| 支持 |

### 分类预测

1. [DNNClassifier](https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/dnnclassifier.py) 基于神经网络的模型，
可以用来做二分类或多分类的预测。
1. [XGBoost Binary Classification](#) 基于树模型的二分类模型。
1. [XGBoost Multiple Classification](#) 基于树模型的多分类模型。

| 模型| 训练速度 | 图形化模型解释 | 是否支持大规模数据集 |
| -- | -- | -- | -- |
| DNNClassifier | 慢 | 支持 | 支持 |
| XGBoost Binary Classification |快|支持| 支持|
| XGBoost Multiple Classification|快|支持| 支持|
