# 信用卡欺诈检测

本教程以信用卡欺诈数据集为例，演示如何使用 SQLFlow 完成分类任务。

## 数据集简介

对银行来说，能够识别欺诈性的信用卡交易是至关重要的，它可以帮助客户避免不必要的经济损失。
[Kaggle信用卡欺诈数据集](https://www.kaggle.com/mlg-ulb/creditcardfraud)包含2013年9月欧洲持卡人通过信用卡进行的交易。此数据集显示两天内发生的交易，在284807笔交易中，共有492笔盗刷。数据集高度不平衡，正例（发生盗刷）占所有交易的0.172%，这要比常见的点击率预估数据集中正例所占的比例（一般在1%左右）还要低。
在这份数据集中，几乎所有特征都是PCA变换之后的数值。因为信用卡的真实数据涉及用户隐私，一般来说都是公司或银行的高度机密，所以该数据集并未提供原始特征和更多有关数据的背景信息。特征V1、V2……V28是通过PCA得到的主成分，只有time和amount两个特征没有被PCA转化。特征time表示每笔交易与数据集中第一笔交易之间经过的时长（以秒为单位）。特征amount是交易金额，此特征可用于设置样本权重。字段class是标签，如果存在盗刷，则该字段值为1，否则为0。

### 导入数据

我们已经将所需数据导入到 MySQL 数据库中，如果您希望从头开始导入一次，可以参考下面的步骤。

#### 下载

Kaggle信用卡欺诈数据集在Kaggle网站上以[压缩包](https://www.kaggle.com/mlg-ulb/creditcardfraud/download)形式提供，需要注册方能下载。下载完成后通过以下命令解压，压缩包中只有一个文件`creditcard.csv`。

```bash
unzip creditcard.csv.zip
```

#### 建表

我们在SQLFlow command-line上用以下语句建立数据表：

```
CREATE DATABASE IF NOT EXISTS creditcard;

CREATE TABLE creditcard.creditcard(
    time int,v1 FLOAT,v2 FLOAT,v3 FLOAT,
    v4 FLOAT,v5 FLOAT,v6 FLOAT,v7 FLOAT,
    v8 FLOAT,v9 FLOAT,v10 FLOAT,v11 FLOAT,
    v12 FLOAT,v13 FLOAT,v14 FLOAT,v15 FLOAT,
    v16 FLOAT,v17 FLOAT,v18 FLOAT,v19 FLOAT,
    v20 FLOAT,v21 FLOAT,v22 FLOAT,v23 FLOAT,
    v24 FLOAT,v25 FLOAT,v26 FLOAT,v27 FLOAT,
    v28 FLOAT,amount FLOAT, class int);
```

#### 导入CSV

建表完成后，将creditcard.csv的内容导入数据表。导入数据时，请指定`csv`文件的绝对路径，
你可以通过 `mysql client` 来执行以下语句。

```
LOAD DATA LOCAL INFILE '/path/to/creditcard.csv'
INTO TABLE creditcard.creditcard CHARACTER SET 'utf8'
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '\"';
```

## 数据分析

我们使用如下的 SQL 语句来查看正负类数据的总条数：

```sql
%%sqlflow
select class, count(*) from creditcard.creditcard group by class;
```

从上面可以看出，负类（无盗刷情况）的样例远远多于正类（有盗刷）的样例。针对这种数据不平衡的情况，我们可以用重采样等方式来做预处理。 下面，我们用一种非常简单的下采样来使训练数据中的正负例数量保持一致。下面的 SQL 语句将数据库中的正例和负例进行打乱，然后将全部正例和等量的负例筛选到`creditcard.train`表中。

```sql
%%sqlflow
DROP TABLE IF EXISTS creditcard.train;

CREATE TABLE creditcard.train AS
SELECT amount,
       v1, v2, v3, v4, v5, v6, v7, v8, v9, v10, v11, v12, v13, v14, v15, 
       v16, v17, v18, v19, v20, v21, v22, v23, v24, v25, v26, v27, v28, class
FROM (
    SELECT * FROM (
        SELECT rand() as r, creditcard.creditcard.*
        FROM creditcard.creditcard
        ORDER BY class DESC, r
        LIMIT 984
    ) t ORDER BY r
) tmp
```

上述 SQL 运行之后，我们可以查看`train`表中的数据分布，结果表明我们选取了等量的正负例。

```sql
%%sqlflow
select class, count(*) from creditcard.train group by class;
```

```txt
+-------+----------+
| class | count(*) |
+-------+----------+
|     0 |      492 |
|     1 |      492 |
+-------+----------+
```

另外，深度模型对数值范围比较敏感。该数据集中 `v` 开头的列数据范围都较小，而 `amount` 的数据范围较大，我们可以对其进行归一化。 读者也可以尝试对其他字段进行归一化。

```txt
SELECT MIN(amount),  MAX(amount) FROM creditcard.creditcard;
+-------------+-------------+
| MIN(amount) | MAX(amount) |
+-------------+-------------+
|           0 |     25691.2 |
+-------------+-------------+
```

注意到 `amount` 范围在 0 ~ 25691.2 之间，我们将每条数据的 `amount` 字段都除以 25691.2 即可。

```sql
%%sqlflow
UPDATE creditcard.train SET amount = amount / 25691.2;
```

## 训练DNNClassifier

执行如下语句，我们选取大约 80% 的数据（表中前780条）来训练一个深度分类器，然后将训练好的模型存储在 `creditcard.my_fraud_dnn_model` 表中。

```sql
%%sqlflow
SELECT * FROM creditcard.train limit 780
TO TRAIN DNNClassifier
WITH train.batch_size=128,
    model.n_classes=2,
    model.batch_norm=True,
    model.hidden_units=[64, 32],
    optimizer.learning_rate=0.1,
    train.epoch=20,
    validation.select="select * from creditcard.train limit 780, 200"
LABEL class
INTO creditcard.my_fraud_dnn_model;
```

我们可以从日志链接中看到训练效果，由于训练集数据具有一定随机性，您所看到的结果可能稍有不同：

```json
{'accuracy': 0.86764705, 'accuracy_baseline': 0.51960784, 'auc': 0.9389199, 'auc_precision_recall': 0.9131467, 'average_loss': 0.43539605, 'label/mean': 0.51960784, 'loss': 0.42728242, 'precision': 0.816, 'prediction/mean': 0.6484489, 'recall': 0.9622642, 'global_step': 500}
```

### 语法简介

在SQLFlow中，SQL语句以标准SQL中的`SELECT`开始，以`TO`为界，`TO`之后就是SQLFlow的扩展SQL语法。在上面的例子中，`TO TRAIN`表示这是一条训练语句，`DNNClassifier`表示我们使用TensorFlow canned estimators中的`DNNClassifier`来针对信用卡欺诈检测数据集进行模型训练。
`LABEL`关键字指定标签字段为class。
`INTO`关键字指定最终构建的模型的名称，同时也是模块存储的表名，这里指定的是my_fraud_dnn_model。训练完成之后我们可以在该表中查询到以Base64格式保存的模型数据。

#### WITH子句

`WITH`子句用于指定模型参数，在上面的例子中，我们指定了五个参数，分别介绍如下：

##### model.n_classes

该参数指定`DNNClassifier`这样的分类模型的类目数量，由于信用卡欺诈数据集的标签class只有盗刷/正常两种情况，所以我们指定类目数量为2。
实际上，2是`DNNClassifier`默认的类目数量，因此例子中的`model.n_classes=2,`是可以省略的。

##### model.hidden_units

该参数指定`DNNClassifier`中神经网络的结构，[200, 100, 50]表示这个`DNNClassifier`具有三个隐藏神经元层，各层的节点数分别为200、100和50。

##### model.batch_norm

该参数设置为True的话，`DNNClassifier`中神经网络的每个隐藏层后会增加一个batch normalization层，上文中提过，V1、V2…V28取值范围较小，但仍然远大于[-1, 1]这个范围，既然我们没有针对数据本身做归一化，则开启batch normalization对效果和训练的稳定性会有很大帮助。

##### train.batch_size

该参数指定`DNNClassifier`训练的batch大小，神经网络一般都采用随机梯度下降算法，每从数据中读取一个batch的大小，就更新一次模型，前面提到，信用卡欺诈数据集的正负例很不均衡，这个参数需要设置得较大，以此来保障每个batch在概率上至少有一条正例，从而保证训练过程能正常执行。
通过对负例进行采样可以在一定程度上解决正负例不均衡的问题，我们把这个练习留给读者。

##### optimizer.learning_rate

optimizer指定TensorFlow的随机梯度下降算法类别，对DNN来说，一般默认为`Adagrad`算法，learning_rate控制算法的学习率，大部分情况下，这是DNN中最重要的超参数。一般来说，较大的batch_size和batch normalization都有助于设置更大的learning_rate。
AdaGrad的默认学习率为0.001。从理论上讲，AdaGrad的学习率应该尽可能的大，但不能太大。实际上，可以将学习率逐步调大来尝试得到更好的性能。在这个例子中，为了讲述方便，我们并不遵循这个原则，而是把学习率直接提高100倍。

## 预测
我们可以通过下面的 SQL 语句来对数据进行预测，这里我们将数据中后 20% 的样本用于预测。

```sql
%%sqlflow
SELECT * from creditcard.train limit 780, 1000
TO PREDICT creditcard.predict.class
USING creditcard.my_fraud_dnn_model;
```

我们可以通过下面的语句来查看预测结果：

```sql
%%sqlflow
SELECT * from creditcard.predict limit 5
```

## 解释
在处理实际问题的时候，并不是用机器学习技术训练出一个效果良好的模型，预测出结果就万事大吉。我们经常需要分析模型学到了什么，以确定模型是如何决策的，模型的预测结果是不是合理的，等等。下面，我们将通过`TO EXPLAIN`语句来解释当前数据集中各个字段对于最终分类结果的影响。

```sql
%%sqlflow
SELECT * FROM creditcard.train WHERE class=1 limit 10
TO EXPLAIN creditcard.my_fraud_dnn_model
WITH summary.plot_type="bar";
```

经过几分钟的运行，你将看到如下的结果，图中将显示各个特征在所选样例分类中的重要性。
![](./figures/credit_card_fraud_explain.png)

# 总结

在这篇文章中，我们介绍了Kaggle信用卡欺诈数据集。接着，我们以`DNNClassifier`为例，介绍了如何使用SQLFlow来完成机器学习的主要任务：

1. 训练：如何根据数据构建模型
    - `SELECT ... TO TRAIN`语法

2. 调参：如何设置不同模型的不同参数：
    - `WITH`子句用于设置训练参数
    - `model.hidden_units`参数用于控制DNN模型的结构
    - `optimizer.learning_rate`参数用于控制DNN模型的学习速率
    - `train.batch_size`参数用于控制模型批次使用数据时的分批大小
    - `train.epoch`参数用于控制模型的训练轮数

3. 预测：在新数据集上应用训练好的模型
    - `SELECT ... TO PREDICT`语法

4. 解释
    - `SELECT ... TO EXPLAIN`语法
