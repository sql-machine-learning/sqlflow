# Run SQLFlow REPL

In addition to buliding SQLFlow into a gRPC server, accessed via Jupyter Notebook, we could also build it into a command line program, whose `main` function reads SQL statements from the console, evaluates them by calling SQLFlow, and prints the results.  This command-line program makes it easy to debug and profile locally without starting the SQLFlow server and the Jupyter server.  We call this command-line program the SQLFlow REPL. SQLFlow REPL supports automatic code completion to ease applying the underlying powerful AI toolset of SQLFlow.

![](figures/repl.gif)

## Quick Start
The SQLFlow Docker image contains the REPL command-line program.  We can run an example session by typing the following command on MacOS.  If you run Docker on Linux, please change `host.docker.internal:3306` to `localhost:3306`.

```
docker run -it --rm --net=host sqlflow/sqlflow repl \
--datasource="mysql://root:root@tcp(host.docker.internal:3306)/?maxAllowedPacket=0"
```

You should be able to see the following:

```
Welcome to SQLFlow.  Commands end with ;

sqlflow>
```

Suppose that we are going to train a DNNClassifier model.

Let's go over some training data from the Iris database:

```sql
sqlflow> SELECT * from iris.train limit 2;
-----------------------------
+--------------+-------------+--------------+-------------+-------+
| SEPAL LENGTH | SEPAL WIDTH | PETAL LENGTH | PETAL WIDTH | CLASS |
+--------------+-------------+--------------+-------------+-------+
|          6.4 |         2.8 |          5.6 |         2.2 |     2 |
|            5 |         2.3 |          3.3 |           1 |     1 |
+--------------+-------------+--------------+-------------+-------+
```

Then we can train a TensorFlow [DNNClassifier](https://www.tensorflow.org/api_docs/python/tf/estimator/DNNClassifier) model using the following statement.

```sql
sqlflow> SELECT *
FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes = 3, model.hidden_units = [10, 20]
COLUMN sepal_length, sepal_width, petal_length, petal_width
LABEL class
INTO sqlflow_models.my_dnn_model;

...
Training set accuracy: 0.96721
Done training
```

To predict using the trained model, we can type the following statement.

```sql
sqlflow> SELECT *
FROM iris.test
TO PREDICT iris.predict.class
USING sqlflow_models.my_dnn_model;

...
Done predicting. Predict table : iris.predict
```

We can then check the prediction result.

```sql
sqlflow> SELECT * from iris.predict limit 3;
...
+--------------+-------------+--------------+-------------+-------+
| SEPAL LENGTH | SEPAL WIDTH | PETAL LENGTH | PETAL WIDTH | CLASS |
+--------------+-------------+--------------+-------------+-------+
|          6.3 |         2.7 |          4.9 |         1.8 |     2 |
|          5.7 |         2.8 |          4.1 |         1.3 |     1 |
|            5 |           3 |          1.6 |         0.2 |     0 |
+--------------+-------------+--------------+-------------+-------+
```

Congratulations! Now you have successfully completed a session using SQLFlow syntax to train model using DNNClassifier and make a quick prediction.

## Command Line Options

|             Option                      | Description |
|-----------------------------------------|-------------|
| -e \<quoted-query-string\>              | Execute from command line without entering interactive mode. e.g. <br>`-e "SELECT * FROM iris.train TRAIN DNNClassifier..." `<br>does the same thing as the training example above.|
| -f \<filename\>                         | Execute from file without entering interactive mode. e.g. <br>`-f ./my_sqlflow.sql`<br>does the same thing as<br>`< ./my_sqlflow.sql` and `cat ./my_sqlflow.sql \| REPL...` |
| -model_dir \<local-directory\>          | Save model to a local directory. e.g. `-model_dir "./models/"` |
| -datasource \<database-connection-url\> | Connect to the specified database. e.g. `-datasource "mysql://root:root@tcp(host.docker.internal:3306)/" ` |

## Keyboard Shortcuts

### Moving the cursor

| Keyboard Shortcut   |                   Action                   |
|---------------------|--------------------------------------------|
|<kbd>Ctrl + a</kbd>  | Go to the **beginning** of the line (Home) |
|<kbd>Ctrl + e</kbd>  | Go to the **end** of the line (End)        |
|<kbd>Ctrl + p</kbd>  | **Previous** statement (Up arrow)          |
|<kbd>Ctrl + n</kbd>  | **Next** statement (Down arrow)            |
|<kbd>Meta + b</kbd>  | Go back one **word**                       |
|<kbd>Meta + f</kbd>  | Go forward one **word**                    |
|<kbd>Ctrl + b</kbd>  | Go back one **character** (Left arrow)     |
|<kbd>Ctrl + f</kbd>  | Go forward one **character** (Right arrow) |

### Editing the statements

| Keyboard Shortcut   |                        Action                          |
|---------------------|--------------------------------------------------------|
|<kbd>Ctrl + l</kbd>  | Clear the screen                                       |
|<kbd>Meta + Del</kbd>| Delete the **word before** the cursor                  |
|<kbd>Meta + d</kbd>  | Delete the **word after** the cursor                   |
|<kbd>Ctrl + w</kbd>  | Delete the **word before** the cursor                  |
|<kbd>Ctrl + d</kbd>  | Delete the **character under** the cursor              |
|<kbd>Ctrl + h</kbd>  | Delete the **character before** the cursor (Backspace) |
|<kbd>Ctrl + k</kbd>  | Delete the **line after** the cursor                   |
|<kbd>Ctrl + u</kbd>  | Delete the **line before** the cursor                  |
|<kbd>  TAB   </kbd>  | Auto completion for model/attributes names             |
