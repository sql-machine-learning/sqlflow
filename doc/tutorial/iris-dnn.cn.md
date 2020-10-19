# 使用DNN对iris数据集进行分类

[![Open In PAI-DSW](https://pai-public-data.oss-cn-beijing.aliyuncs.com/EN-pai-dsw.svg)](https://dsw-dev.data.aliyun.com/?fileUrl=http://cdn.sqlflow.tech/sqlflow/tutorials/latest/iris-dnn.ipynb&fileName=sqlflow_tutorial_iris_dnn.ipynb)

本文档介绍如何：
- 基于[iris数据集](https://en.wikipedia.org/wiki/Iris_flower_data_set)训练DNNClassifier
- 用训练好的DNNClassifier来预测iris(鸢尾花)的三个亚种(山鸢尾、杂色鸢尾、维吉尼卡鸢尾)

## 数据集简介

iris数据集包含四个特征及一个标签。四个特征表示每株鸢尾花的植物学形状，每个特征是个浮点数。标签代表每株鸢尾花的亚种，是个整数，取值为0、1或2。

在SQLFlow官方镜像里，iris数据集存储在`iris.train`和`iris.test`中，分别是训练数据和测试数据。

iris是一个很小的数据集，您可以运行以下语句查看数据内容：

```sql
%%sqlflow
describe iris.train;
```

```sql
%%sqlflow
select * from iris.train limit 5;
```

## 训练

我们在本节中训练一个三分类的DNNClassifier，它包含两个隐藏层，每层10个节点。使用SQLFlow扩展语法提供的train子句，我们可以很容易地指定模型结构：

```sql
TO TRAIN DNNClassifier
WITH
  model.n_classes = 3,
  model.hidden_units = [10, 10]
```

而通过标准SQL语句`SELECT * FROM iris.train`便可以指定训练数据。

如果要显式指定特征和标签对应的数据列，则可以通过COLUMN子句来完成：

```sql
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
```

通过`INTO sqlflow_models.my_dnn_model`子句，在训练结束时，我们便可将训练好的DNN模型存入数据表`sqlflow_models.my_dnn_model`中。

把以上所有片段结合起来，我们就得到了一条完整的SQLFlow训练语句：

```sql
%%sqlflow
SELECT * FROM iris.train TO TRAIN DNNClassifier WITH
  model.n_classes = 3,
  model.hidden_units = [10, 10],
  train.epoch = 10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
```

以上语句中，第一行的`%%sqlflow`是Jupyter notebook的magic指令，如果您使用SQLFlow命令行或其它工具，则不需要键入这一指令。

上述训练语句一般几分钟内可以运行完毕，SQLFlow给出的典型输出如下：

```python
{'accuracy': 0.4, 'average_loss': 1.0920922, 'loss': 1.0920922, 'global_step': 1100}
```
如您所见，这条训练语句的平均loss并非十分理想，因为*iris数据集*上训练得到的理想结果一般小于0.4。接下来我们来学习如何改进模型效果。

## 模型调优

为了改进模型性能，我们可以手动调整模型的超参数([hyperparameters](https://en.wikipedia.org/wiki/Hyperparameter_(machine_learning)))。
> 在机器学习中，超参数是指学习过程开始前可以指定的参数，而其它参数则是在训练过程中学到的。

根据万能近似理论([Universal approximation theorem](https://en.wikipedia.org/wiki/Universal_approximation_theorem))，一个像DNNClassifier这样的多层前馈网络([feed-forward network](https://en.wikipedia.org/wiki/Feedforward_neural_network))，设计强大的网络结构可使其有潜力模拟任何函数。

我们的第一个效果优化的尝试就是调整网络结构：把每个隐藏层的节点数从10调整为100。这是因为在万能近似理论中，前馈网络的宽度对结果的准确程度有很大影响。

```sql
%%sqlflow
SELECT * FROM iris.train TO TRAIN DNNClassifier WITH
  model.n_classes = 3,
  model.hidden_units = [100, 100],
  train.epoch = 10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
```

上述语句运行结束后，我们发现模型效果有一定改进，loss缩小了近50%：

```python
{'accuracy': 0.72, 'average_loss': 0.5695601, 'loss': 0.5695601, 'global_step': 1100}
```

当然，DNN的表达能力极高，对iris这样的小数据集来说，我们还有不少空间可以改进。

我们的第二个效果优化尝试是增大`DNNClassifier`底层数值优化器的学习率([learning rate](https://en.wikipedia.org/wiki/Learning_rate))，以此来加速学习过程。对DNN来说，优化器及其学习率可能是最为关键的超参数。`DNNClassifier`默认的优化器是[AdaGrad](https://en.wikipedia.org/wiki/Stochastic_gradient_descent#AdaGrad)，其默认学习率为0.001。

理论上说，在神经元不死(参见[dying neuron problem](https://en.wikipedia.org/wiki/Rectifier_(neural_networks)#Potential_problems))的前提下，AdaGrad的学习率可以尽量调大。我们先将其调到原来的10倍：

```sql
%%sqlflow
SELECT * FROM iris.train TO TRAIN DNNClassifier WITH
  model.n_classes = 3,
  model.hidden_units = [100, 100],
  optimizer.learning_rate=0.1,
  train.epoch = 10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
```

上述语句给出的结果明显有很大改善：

```python
{'accuracy': 0.98, 'average_loss': 0.10286382, 'loss': 0.10286382, 'global_step': 1100}
```

在实际工作中，略微调大AdaGrad的学习率往往能带来效果上的提升。

关于手动调优，本文主要就介绍这些内容。实际上，调参在机器学习工作的重要性非常之高，一般会占据整条链路中最多的工时。

## 自动调优

如果您觉得为机器学习模型调参实在是枯燥无味，也可以考虑借助[AutoML](https://en.wikipedia.org/wiki/Automated_machine_learning)技术来自动进行调优。

SQLFlow通过特定的estimator提供了基于NAS([neural architecture search](https://en.wikipedia.org/wiki/Neural_architecture_search))的自动调优能力。我们可以使用`sqlflow_models.AutoClassifier`代替`DNNClassifier`来实现自动调优。一旦使用`sqlflow_models.AutoClassifier`，就不需要再关注网络结构，因此不必在`WITH`子句中指定`hidden_unit`，而默认的学习率也能够应付大部分情况。

```sql
%%sqlflow
SELECT * FROM iris.train TO TRAIN sqlflow_models.AutoClassifier WITH
  model.n_classes = 3, train.epoch = 10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
```

上述语句运行时间要比之前的那些语句更久，因为`AutoClassifier`需要搜索最合适的网络架构。这条语句运行结束后输出类似如下结果：

```python
{'accuracy': 0.98, 'average_loss': 0.08678584, 'loss': 0.08678584, 'global_step': 1000}

```

尽管这个结果看起来和手动调优的结果非常接近，但因为`DNNClassifier`和`AutoClassifier`都有一定随机性，您自己运行得到的结果可能会略有不同。

SQLFlow项目组计划在未来支持更多的NAS模型，同时也计划支持HPO([automatic hyperparameter tuning](https://en.wikipedia.org/wiki/Automated_machine_learning#Hyperparameter_optimization_and_model_selection))等其它AutoML技术。

## 预测

SQLFlow提供了非常易用的预测能力。和训练语句一样，我们使用`SELECT * FROM iris.test`这样的标准SQL语句来指定预测数据。

如果我们想用先前训练的模型`sqlflow_models.my_dnn_model`来预测上述数据，并将结果写到`iris.predict`表的`class`这列，则完整的预测语句如下：

```sql
%%sqlflow
SELECT * FROM iris.test TO PREDICT iris.predict.class USING sqlflow_models.my_dnn_model;
```
预测语句运行结束后，我们就可以通过如下标准SQL语句来查看预测结果：

```sql
%%sqlflow
SELECT * FROM iris.predict LIMIT 5;
```
