# Classify Iris Dataset Using DNNClassifer

<a href="https://dsw-dev.data.aliyun.com/?fileUrl=http://cdn.sqlflow.tech/sqlflow/tutorials/latest/iris-dnn.ipynb&fileName=sqlflow_tutorial_iris_dnn.ipynb">
  <img alt="Open In PAI-DSW" src="https://pai-public-data.oss-cn-beijing.aliyuncs.com/EN-pai-dsw.svg">
</a>

This tutorial demonstrates how to
- Train a DNNClassifer on the [Iris flower dataset](https://en.wikipedia.org/wiki/Iris_flower_data_set).
- Use the trained DNNClassifer to predict the three species of Iris(Iris setosa, Iris virginica and Iris versicolor).

## The Dataset

The Iris data set contains four features and one label. The four features identify the botanical characteristics of individual Iris flowers. Each feature is stored as a single float number. The label indicates the species of individual Iris flowers. The label is stored as a integer and has possible value of 0, 1, 2.

We have prepared the Iris dataset in table `iris.train` and `iris.test`. We will use them as training data and test data respectively.

We can have a quick peek of the data by running the following standard SQL statements.

```sql
%%sqlflow
describe iris.train;
```

```sql
%%sqlflow
select * from iris.train limit 5;
```

## Train

Let's train a ternary DNNClassifier, which has two hidden layers with ten hidden units each. This can be done by specifying the training clause for SQLFlow's extended syntax.

```
TO TRAIN DNNClassifier
WITH
  model.n_classes = 3,
  model.hidden_units = [10, 10]
```

To specify the training data, we use standard SQL statements like `SELECT * FROM iris.train`.

We explicit specify which column is used for features and which column is used for the label by writing

```
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
```

At the end of the training process, we save the trained DNN model into table `sqlflow_models.my_dnn_model` by writing `INTO sqlflow_models.my_dnn_model`.

Putting it all together, we have our first SQLFlow training statement.

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

The above training statement usually takes a few minutes to run, and the outputs look like the following:

```python
{'accuracy': 0.4, 'average_loss': 1.0920922, 'loss': 1.0920922, 'global_step': 1100}
```

As we've seen, the average loss of the above training statement doesn't look very good; an ideal value for the *Iris flower dataset* should be less 0.4. Let us see what we can do to improve model quality.

## Tune

In order to improve the model performance, we can tune the [hyperparameters](https://en.wikipedia.org/wiki/Hyperparameter_(machine_learning)) manually.
> In machine learning, a hyperparameter is a parameter whose value is set before the learning process begins. By contrast, the values of other parameters are derived via training.

According to the [Universal approximation theorem](https://en.wikipedia.org/wiki/Universal_approximation_theorem), the architecture of a multilayer [feed-forward network](https://en.wikipedia.org/wiki/Feedforward_neural_network) (such as our `DNNClassifier`) gives the neural network the potential of being a universal approximator.

Our first *performance improvement trial* is to tune the architecture of our model by increasing the `hidden_units` of each layer to 100 because the width of feed-forward networks matters in the theorem.

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
The above statement will give a better result like:

```python
{'accuracy': 0.72, 'average_loss': 0.5695601, 'loss': 0.5695601, 'global_step': 1100}
```

However, DNNs are highly expressive models, for our tiny dataset, we still have a lot of room for improvement.

Our second *performance improvement trial* is to enlarge the [learning rate](https://en.wikipedia.org/wiki/Learning_rate) of the underlying optimizer of `DNNClassifier` to speed up the learning process. Optimizers and the learning rate are the the most important hyperparameters in DNNs. The default optimizer of `DNNClassifier` is [AdaGrad](https://en.wikipedia.org/wiki/Stochastic_gradient_descent#AdaGrad) with a default learning rate of 0.001.

Theoretically speaking, the learning rate of AdaGrad should be set as large as possible, but no larger. Practically speaking, a slightly larger learning rate always makes AdaGrad perform slightly better as long as the [dying neuron problem](https://en.wikipedia.org/wiki/Rectifier_(neural_networks)#Potential_problems) doesn't arise. Let us increase the learning rate by 10 times:

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
The above statement will give a decent result like:

```python
{'accuracy': 0.98, 'average_loss': 0.10286382, 'loss': 0.10286382, 'global_step': 1100}

```

That's all you have to know about tuning models in this tutorial. In fact, tuning is very crucial to make machine learning work and usually takes a large fraction of the working hours of data scientists and machine learning engineers.


## Automated tuning

If you feel that tuning models manually is time-consuming and tedious (it is indeed), [automated machine learning](https://en.wikipedia.org/wiki/Automated_machine_learning) (AutoML) can be a fine alternative.

SQLFlow supports automated [neural architecture search](https://en.wikipedia.org/wiki/Neural_architecture_search) (NAS) via specific estimators. To improve model performance, we can use `sqlflow_models.AutoClassifier` instead of `DNNClassifier`. We don't need to specify the `hidden_units` in the `WITH` clause in the above example because `sqlflow_models.AutoClassifier` will automatically search for the architecture.
```sql
%%sqlflow
SELECT * FROM iris.train TO TRAIN sqlflow_models.AutoClassifier WITH
  model.n_classes = 3,
  train.epoch = 10
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
```
The above training statement will take longer to run because the `AutoClassifier` has to search for the most appropriate neural architectures. The statement will give a result like:

```python
{'accuracy': 0.98, 'average_loss': 0.08678584, 'loss': 0.08678584, 'global_step': 1000}

```

Although this seems to be very close to the manually tuned version in the last section, because the training process of `DNNClassifier` and `AutoClassifier` is somewhat stochastic, it may give an average loss slightly large or smaller than the manually tuned version.

The SQLFlow team plans to support more NAS models as well as other AutoML technics like [automatic hyperparameter tuning](https://en.wikipedia.org/wiki/Automated_machine_learning#Hyperparameter_optimization_and_model_selection) in the near future.


## Predict

SQLFlow also supports prediction out-of-the-box.

To specify the prediction data, we use standard SQL statements like `SELECT * FROM iris.test`.

Say we want the model, previously stored at `sqlflow_models.my_dnn_model`, to read the prediction data and write the predicted result into table `iris.predict` column `class`. We can write the following SQLFlow prediction statement.

```sql
%%sqlflow
SELECT * FROM iris.test TO PREDICT iris.predict.class USING sqlflow_models.my_dnn_model;
```

After the prediction, we can checkout the prediction result by

```sql
%%sqlflow
SELECT * FROM iris.predict LIMIT 5;
```
