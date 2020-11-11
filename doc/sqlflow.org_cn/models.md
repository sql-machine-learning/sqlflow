
# 模型手册

本章节会详细介绍目前SQLFlow目前可用的模型以及模型可配置的详细参数。每个模型可以使用如下的语法使用：

```sql
SELECT ...
TO TRAIN [模型名]
WITH 参数1=值1,参数2=值2, ...
LABEL ...
INTO ...
```

# Estimator模型
## DNNClassifier

使用DNN结构的分类器模型：https://www.tensorflow.org/api_docs/python/tf/estimator/DNNClassifier

可以支持的参数包含 (WITH) :

| 参数 | 说明 |
| -------- | -------- |
| model.hidden_units     | DNN模型中每个隐层的神经元个数，如[128,32] 表示模型有2个隐层，神经元个数分别是128, 32  |
| model.n_classes | 模型可分类的类别总数(分类器模型使用)，比如：2 表示模型将把数据分为2类 |
| model.optimizer | 配置模型训练使用的optimizer，支持的optimizer参考：[Optimizer配置方法](https://yuque.antfin-inc.com/sqlflownews/userguide/params#827c4728) |
| model.batch_norm | 是否在每个隐层之后使用batch_norm，比如：True表示开启，默认为False |
| model.dropout | 配置dropout概率，比如：0.5，默认为None |
| model.activation_fn | 待验证 |

## DNNRegressor

可以训练使用DNN结构的回归类模型，可配置的参数参考：[DNNClassifier](#DNNClassifier)

## LinearClassifier
线性分类器模型： https://www.tensorflow.org/api_docs/python/tf/estimator/LinearClassifier

可以支持的参数包括 （WITH）：

| 参数 | 说明 |
| -------- | -------- |
| model.n_classes | 模型可分类的类别总数(分类器模型使用)，比如：2 表示模型将把数据分为2类 |
| model.optimizer | 配置模型训练使用的optimizer，支持的optimizer参考：[Optimizer配置方法](https://yuque.antfin-inc.com/sqlflownews/userguide/params#827c4728) |
| model.sparse_combiner | 指定如果category column输入如果包含多个相同值时的处理方法，可以是："mean", "sqrtn", 或 "sum" |

## LinearRegressor
线性回归模型：https://www.tensorflow.org/api_docs/python/tf/estimator/LinearRegressor
支持的参数参考：[LinearClassifier](#LinearClassifier)

## BoostedTreesClassifier
Tensorflow Boosted Trees分类器模型：https://www.tensorflow.org/api_docs/python/tf/estimator/BoostedTreesClassifier

可以支持的参数包括（WITH）：

| 参数 | 说明 |
| -------- | -------- |
| model.n_classes | 模型可分类的类别总数(分类器模型使用)，目前该模型只支持2分类 |
| model.n_batches_per_layer | 在训练每层时使用多少batch数据计算统计值 |
| model.n_trees | 模型创建树的总数，如：100 |
| model.max_depth | 每棵树的最大深度，如：6 |
| model.learning_rate | 学习率，如：0.3 |
| model.l1_regularization | l1正则项，默认为0.0 |
| model.l2_regularization | l2正则项，默认为0.0 |
| model.tree_complexity | 树的叶子结点居多时的惩罚正则项 |
| model.center_bias | 是否开启偏置剧中，True为开启，默认为False，如果后续需要执行模型解释，则需要开启此项 |


## BoostedTreesRegressor
Tensorflow Boosted Trees回归模型：https://www.tensorflow.org/api_docs/python/tf/estimator/BoostedTreesRegressor

可支持的参数参考： [BoostedTreesClassifier](#BoostedTreesClassifier)

## DNNLinearCombinedClassifier
训练Deep Wide分类器模型：https://www.tensorflow.org/api_docs/python/tf/estimator/DNNLinearCombinedClassifier

可支持的参数包含（WITH）：

| 参数 | 说明 |
| -------- | -------- |
| model.linear_optimizer | 模型线性部分使用的优化器，详细参考：[Optimizer配置方法](https://yuque.antfin-inc.com/sqlflownews/userguide/params#827c4728) |
| model.dnn_optimizer | 模型DNN部分使用的优化器，详细参考：[Optimizer配置方法](https://yuque.antfin-inc.com/sqlflownews/userguide/params#827c4728) ｜
| model.dnn_hidden_units | 模型DNN部分隐层每层的神经元数量，如：[128,32]表示包含2个隐层，每层的神经元个数分别为128, 32 |
| model.dnn_dropout | 模型DNN部分每个隐层后是否执行dropout，默认为None |
| model.n_classes | 模型可分类的类别总数(分类器模型使用) |
| model.batch_norm | 是否在每个隐层之后使用batch_norm，比如：True表示开启，默认为False |
| model.linear_sparse_combiner | 指定如果category column输入如果包含多个相同值时的处理方法，可以是："mean", "sqrtn", 或 "sum" |

## DNNLinearCombinedRegressor

训练Deep Wide回归模型：https://www.tensorflow.org/api_docs/python/tf/estimator/DNNLinearCombinedRegressor

可支持的参数参考：[DNNLinearCombinedClassifier](#DNNLinearCombinedClassifier)

# XGBoost模型
## xgboost.gbtree
使用XGBoost训练Gradient boosting tree 模型，更详细的参数说明，参考：https://xgboost.readthedocs.io/en/latest/parameter.html#parameters-for-tree-booster

| 参数 | 说明 |
| -------- | -------- |
| train.num_boost_round | 训练轮数，默认为1 |
| objective | 必选，指定训练的目标。二分类一般使用"binary:logistic", 多分类使用 "multi:softmax"，回归使用 "reg:squarederror"，详细参考：https://xgboost.readthedocs.io/en/latest/parameter.html#learning-task-parameters |
| num_class | 多分类时，类别的数量 |
| eta | 学习率（learning_rate），比如：0.3 |
| gamma | 拆分叶子节点需要的最小loss reduction |
| max_depth | 最大树深度 |
| min_child_weight | Minimum sum of instance weight (hessian) needed in a child，默认：1 |
| max_delta_step | Maximum delta step we allow each leaf output to be，默认：0 |
| subsample | Subsample ratio of the training instances， 默认：1|
| colsample_bytree | This is a family of parameters for subsampling of columns，默认：1 |
| colsample_bylevel | This is a family of parameters for subsampling of columns，默认：1 |
| colsample_bynode | This is a family of parameters for subsampling of columns，默认：1 |
| lambda | L2 regularization term on weights. Increasing this value will make model more conservative. |
| alpha | L1 regularization term on weights. Increasing this value will make model more conservative. |
| tree_method | Choices: auto, exact, approx, hist, gpu_hist, this is a combination of commonly used updaters，默认：auto |
| sketch_eps | Only used for tree_method=approx，默认：0.03 |
| scale_pos_weight | Control the balance of positive and negative weights, useful for unbalanced classes，默认：1 |
| updater | A comma separated string defining the sequence of tree updaters to run |
| refresh_leaf | This is a parameter of the refresh updater. |
| process_type | A type of boosting process to run. |
| grow_policy | Controls a way new nodes are added to the tree. |
| max_leaves | Maximum number of nodes to be added. Only relevant when grow_policy=lossguide is set. |
| max_bin | Only used if tree_method is set to hist. |
| predictor | The type of predictor algorithm to use. |
| num_parallel_tree | Number of parallel trees constructed during each iteration. |

示例：
1. 二分类
	```sql
	SELECT * FROM train_table
	TO TRAIN xgboost.gbtree
	WITH objective="binary:logistic", validation.select="SELECT * FROM val_table"
	LABEL class
	INTO my_xgb_classification_model;
	```
3. 多分类
	```sql
	SELECT * FROM train_table
	TO TRAIN xgboost.gbtree
	WITH objective="multi:softmax", num_class=3, validation.select="SELECT * FROM val_table"
	LABEL class
	INTO my_xgb_classification_model;
	```
5. 回归任务
	```sql
	SELECT * FROM train_table
	TO TRAIN xgboost.gbtree
	WITH objective="reg:squarederror", validation.select="SELECT * FROM val_table"
	LABEL target
	INTO my_xgb_regression_model;
	```


## xgboost.dart

使用XGBoost训练DART 模型，模型配置项与xgboost.gbtree相同，**另外还**支持下面的参数：

| 参数 | 说明 |
| -------- | -------- |
| sample_type | Type of sampling algorithm: uniform or weighted |
| normalize_type | Type of normalization algorithm: tree or forest |
| rate_drop | Dropout rate |
| one_drop | When this flag is enabled, at least one tree is always dropped during the dropout |
| skip_drop | Probability of skipping the dropout procedure during a boosting iteration |


## xgboost.gblinear

使用XGBoost训练linear模型，可支持的参数有：

| 参数 | 说明 |
| -------- | -------- |
| lambda | L2 regularization term on weights. Increasing this value will make model more conservative. |
| alpha | L1 regularization term on weights. Increasing this value will make model more conservative. |
| updater | Choice of algorithm to fit linear model: shotgun, coord_descent |
| feature_selector | Feature selection and ordering method: cyclic, shuffle, random, greedy, thrifty |
| top_k | The number of top features to select in greedy and thrifty feature selector. The value of 0 means using all the features. |


# 随机森林模型

## randomforests

| 参数 | 说明 |
| -------- | -------- |
| tree_num     | 生成的树的个数，默认： 1  |

# Keras模型
## sqlflow_models.DNNClassifier

使用Keras实现的示例DNN分类器模型，支持的参数包括：

| 参数 | 说明 |
| -------- | -------- |
| model.hidden_units     | DNN模型中每个隐层的神经元个数，如[128,32] 表示模型有2个隐层，神经元个数分别是128, 32  |
| model.n_classes | 模型可分类的类别总数(分类器模型使用)，比如：2 表示模型将把数据分为2类 |
| model.optimizer | 配置模型训练使用的optimizer，支持的optimizer参考：[Optimizer配置方法](https://yuque.antfin-inc.com/sqlflownews/userguide/params#827c4728) |

## sqlflow_models.StackedBiLSTMClassifier

使用Keras实现的StackedBiLSTM分类器模型，可用于文本分类。支持的参数包括：

| 参数 | 说明 |
| -------- | -------- |
| model.stack_units  | 双向LSTM的每层的大小，如：[32]表示使用一层双向LSTM，LSTM size是32  |
| model.n_classes | 模型可分类的类别总数，比如：2 表示模型将把数据分为2类 |
| model.optimizer | 配置模型训练使用的optimizer，支持的optimizer参考：[Optimizer配置方法](https://yuque.antfin-inc.com/sqlflownews/userguide/params#827c4728) |
| model.hidden_size | 最后隐层的神经元树木，如：64表示使用一个大小为64的隐层链接最后一层LSTM和输出层 |


## sqlflow_models.LSTMBasedTimeSeriesModel

基于LSTM的时间序列预测模型，输入多个时间点的feature，预测未来多个时间点的表现。

| 参数 | 说明 |
| -------- | -------- |
| model.stack_units  |  LSTM层数和每层的size的配置，默认：[500,500] |
| model.n_in | 输入的时间点个数 |
| model.n_out | 输出的时间点个数 |

## sqlflow_models.AutoClassifier (TBD)

TBD

# 无监督模型

## sqlflow_models.DeepEmbeddingClusterModel

无监督模型Deep Embedding Clustering: https://arxiv.org/abs/1511.06335 实现，支持的参数配置包括：

| 参数 | 说明 |
| -------- | -------- |
| model.n_clusters  |  聚类的类别个数，默认：10 |
| model.kmeans_init | 执行kmeans的次数用于获得最佳中心点，默认：20 |
| model.run_pretrain | 是否执行与训练，默认：True |
| model.pretrain_dims | 预训练autoencoder时，各隐层的维度，默认：[500, 500, 2000, 10] |
| model.pretrain_activation_func | autoencoder部分激活函数，默认：'relu' |
| model.pretrain_batch_size | autoencoder与训练的batch size，默认：256 |
| model.train_batch_size | 训练 batch size，默认：256 |
| model.pretrain_epochs | 执行与训练的轮数，默认：10 |
| model.pretrain_initializer | autoencoder参数初始化方法，默认：'glorot_uniform' |
| model.pretrain_lr | 预训练learning rate，默认： 1 |
| model.train_lr | 训练学习率，默认：0.1 |
| model.train_max_iters | 训练最大迭代步数，默认：8000 |
| model.update_interval | 更新目标分布的间隔步数，默认：100 |
| model.tol | tol，默认：0.001 |


## KMeans

无监督聚类：[K-均值](https://en.wikipedia.org/wiki/K-means_clustering)算法，支持的参数配置包括：

|参数|说明|
|----|----|
|center_count | 聚类类别个数，默认：3|
| idx_table_name | 聚类结果输出表|
|excluded_columns| 那些列不作为特征，CSV格式，以`,`分隔|


## DBSCAN (TBD)

