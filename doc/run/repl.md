# Run SQLFlow REPL

In addition to building SQLFlow into a gRPC server, accessed via Jupyter Notebook, we could also build it into a command line program, whose `main` function reads SQL statements from the console, evaluates them by calling SQLFlow, and prints the results.  This command-line program makes it easy to debug and profile locally without starting the SQLFlow server and the Jupyter server.  We call this command-line program the SQLFlow REPL. SQLFlow REPL supports automatic code completion to ease applying the underlying powerful AI toolset of SQLFlow.

![](figures/repl.gif)

## Quick Start

The SQLFlow Docker image contains the REPL command-line program.  We can run an example session by typing the following command on MacOS.  If you run Docker on Linux, please change `host.docker.internal:3306` to `localhost:3306`.

```
docker run -it --rm --net=host sqlflow/sqlflow bash -c 'chmod +x start.sh; ./start.sh populate-example-dataset-mysql-local; \
repl --datasource="mysql://root:root@tcp(host.docker.internal:3306)/?maxAllowedPacket=0"'
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

## Command-line Options

|             Option                      | Description |
|-----------------------------------------|-------------|
| -e \<quoted-query-string\>              | Execute from command line without entering interactive mode. e.g. <br>`-e "SELECT * FROM iris.train TRAIN DNNClassifier..." `<br>does the same thing as the training example above.|
| -f \<filename\>                         | Execute from file without entering interactive mode. e.g. <br>`-f ./my_sqlflow.sql`<br>does the same thing as<br>`< ./my_sqlflow.sql` and `cat ./my_sqlflow.sql \| REPL...` |
| -model_dir \<local-directory\>          | Save model to a local directory. e.g. `-model_dir "./models/"` |
| -datasource \<database-connection-url\> | Connect to the specified database. e.g. `-datasource "mysql://root:root@tcp(host.docker.internal:3306)/" ` |
| -A                                      | No auto completion for `sqlflow_models`. This gives a quicker start |

## Keyboard Shortcuts

### Moving the cursor

|  Keyboard Shortcut  |                   Action                   |
|---------------------|--------------------------------------------|
| <kbd>Ctrl + a</kbd> | Go to the **beginning** of the line (Home) |
| <kbd>Ctrl + e</kbd> | Go to the **end** of the line (End)        |
| <kbd>Meta + b</kbd> | Go back one **word**                       |
| <kbd>Meta + f</kbd> | Go forward one **word**                    |
| <kbd>Ctrl + b</kbd> | Go back one **character** (Left arrow)     |
| <kbd>Ctrl + f</kbd> | Go forward one **character** (Right arrow) |

### Editing

| Keyboard Shortcut     |                        Action                                        |
|-----------------------|----------------------------------------------------------------------|
| <kbd>Ctrl + l</kbd>   | Clear the screen                                                     |
| <kbd>Meta + Del</kbd> | Cut the **word before** the cursor to the clipboard                  |
| <kbd>Meta + d</kbd>   | Cut the **word after** the cursor to the clipboard                   |
| <kbd>Ctrl + w</kbd>   | Cut the **word before** the cursor to the clipboard                  |
| <kbd>Ctrl + d</kbd>   | Delete the **character under** the cursor                            |
| <kbd>Ctrl + h</kbd>   | Delete the **character before** the cursor (Backspace)               |
| <kbd>Ctrl + k</kbd>   | Cut the **line after** the cursor to the clipboard                   |
| <kbd>Ctrl + u</kbd>   | Cut the **line before** the cursor to the clipboard                  |
| <kbd>Ctrl + y</kbd>   | Paste the last thing to be cut (yank)                                |
| <kbd>  TAB   </kbd>   | Auto completion for model/attributes names, navigate the pop-up menu |

### History

| Keyboard Shortcut        |                   Action                                                                          |
|--------------------------|---------------------------------------------------------------------------------------------------|
| <kbd>Ctrl + r</kbd>      | Pop a menu of history commands including the specified character(s), the menu updates as you type |
| <kbd>Ctrl/Meta + p</kbd> | Show the **previous** statement in history, or navigate the pop-up history menu (Up arrow)        |
| <kbd>Ctrl/Meta + n</kbd> | Show the **Next** statement in history, or navigate the pop-up history menu (Down arrow)          |
| <kbd>Meta + W</kbd>      | Similar to <kbd>Ctrl + r</kbd> but use wildcard to search history commands                        |

### Control

|  Keyboard Shortcut  |          Action       |
|---------------------|-----------------------|
| <kbd>Ctrl + L</kbd> | Clear the screen      |
| <kbd>Ctrl + D</kbd> | Exit (when no inputs) |
