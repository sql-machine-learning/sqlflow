# 快速开始

我们以[波士顿房价](https://www.kaggle.com/c/boston-housing)数据集为例，演示如何在笔记本电脑上的 Jupyter Notebook 中使用 SQLFlow 训练模型，并用训练好的模型进行预测，最后分析每个特征对模型的贡献。

## 使用 Docker 启动 SQLFlow

1. 在笔记本上[安装 Docker](https://docs.docker.com/get-docker/)
1. 启动 SQLFlow： `docker run -it -p 8888:8888 sqlflow/sqlflow`
1. 在浏览器中打开页面 `http://localhost:8888`, 随后即可看到 Jupyter Notebook 的页面了。

## 在 Notebook 中创建 Python3 Kernel
![](/doc/usage/figures/py3_kernel.png)

## 训练模型

在 Notebook 中输入以下扩展 SQL 语句并运行：

``` sql
%%sqlflow
SELECT * FROM boston.train
TO TRAIN xgboost.gbtree
WITH
      objective="reg:squarederror",
      train.num_boost_round = 30
LABEL medv
INTO sqlflow_models.my_xgb_regression_model;
```

看到如下日志表示训练任务执行完毕：
![](/doc/usage/figures/quickstart_train.jpg)

## 使用训练好的模型进行预测

完成训练后，输入以下扩展 SQL 语句并运行即可使用训练好的模型进行预测：

``` sql
%%sqlflow
SELECT * FROM boston.test
TO PREDICT boston.predict.medv
USING sqlflow_models.my_xgb_regression_model;
```

看到如下日志即表示预测任务运行完毕：
![](/doc/usage/figures/quickstart_predict.jpg)

我们可以输入一条标准 `SELECT` 语句检查以下预测结果表中的数据，从而确认预测结果已经写入到 `medv` 列中。

``` sql
%%sqlflow
SELECT * FROM boston.predict LIMIT 10;
```

![](/doc/usage/figures/quickstart_select.jpg)

## 分析每个特征对模型的贡献

``` sql
%%sqlflow
SELECT *
FROM boston.train
TO EXPLAIN sqlflow_models.my_xgb_regression_model
WITH
        summary.plot_type="dot",
        summary.alpha=1,
        summary.sort=True
USING TreeExplainer;
```

运行成功后可以在日志框中显示如下直方图，可以看到波士顿房价这个数据集中 “低收入人群占比(LSTAT)” 这个特征对最终房价的影响是最大的:
    
![](/doc/usage/figures/quickstart_explain.jpg)
