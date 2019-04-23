## Demo: Using SQLFlow from Command Line

Besides running SQLFlow from Notebook, we could also run it from command line. If you are using Docker for Linux, please change `host.docker.internal:3306` to `localhost:3306`.

```
docker run -it --rm --net=host sqlflow/sqlflow:latest demo \
--db_user root --db_password root --db_address host.docker.internal:3306
```

You should be able to see the following:

```
sqlflow>
```

### Training a DNNClassifier and run prediction

- Step one: Let's see training data from Iris database:
```sql
sqlflow> select * from iris.train limit 2;
-----------------------------
+--------------+-------------+--------------+-------------+-------+
| SEPAL LENGTH | SEPAL WIDTH | PETAL LENGTH | PETAL WIDTH | CLASS |
+--------------+-------------+--------------+-------------+-------+
|          6.4 |         2.8 |          5.6 |         2.2 |     2 |
|            5 |         2.3 |          3.3 |           1 |     1 |
+--------------+-------------+--------------+-------------+-------+
```

- Step Two: Train a Tensorflow [DNNClassifier](https://www.tensorflow.org/api_docs/python/tf/estimator/DNNClassifier) from train table:
```sql
sqlflow> SELECT *
FROM iris.train
TRAIN DNNClassifier
WITH n_classes = 3, hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;
-----------------------------
...
Training set accuracy: 0.96721
Done training
```

- Step Three: Run prediction from a trained model:
```sql
sqlflow> SELECT *
FROM iris.test
predict iris.predict.class
USING sqlflow_models.my_dnn_model;
```

- Step Four: Checkout the prediction result:
```sql
sqlflow> select * from iris.predict limit 10;
```
Now you have successfully run a demo in SQLFlow to train the model using DNNClassifier and make a simple prediction. More demos are on the road, please stay tuned.