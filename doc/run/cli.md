# Run SQLFlow Command-line Tool

In addition to the Jupyter Notebook magic command, we provide a command-line client `sqlflow` to connect and operate the SQLFlow gRPC server. Similar to other command-line client tools like `mysql`, `sqflow` implements several interaction modes.

1. `sqlflow -e "SELECT ... TO TRAIN ..."` runs the SQLFlow program provided in the command line.
1. `sqlflow -f a.sql` runs the SQLFlow program in a file.
1. `sqflow` starts the REPL mode for user interaction.

In the REPL mode, `sqlflow` supports automatic code completion and other features.

![](figures/repl.gif)

## Installation
You can click below links to download the `sqlflow` binary according to your system type:
- [Linux](http://cdn.sqlflow.tech/latest/linux/sqlflow)
- [macOS](http://cdn.sqlflow.tech/latest/osx/sqlflow)
- [Windows](http://cdn.sqlflow.tech/latest/windows/sqlflow)

For Linux and macOS, after downloading the `sqlflow` binary, you need to change the file's mode to allow execution. e.g.
```bash
wget http://cdn.sqlflow.tech/latest/linux/sqlflow
chmod +x sqlflow
```

## Quick Start

We can run an example session by typing the following command on macOS.  We assume you have installed our [playground](https://github.com/sql-machine-learning/playground/blob/master/dev.md) on your computer.

- follow [this guide](https://github.com/sql-machine-learning/playground/blob/master/dev.md#for-end-users) to start your playground
- connect to SQLFlow server according to the prompt message output by the playground, just do copy & paste, like:
```bash
./sqlflow --data-source="mysql://root:root@tcp(172.17.0.8)/?maxAllowedPacket=0"
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
+--------------+-------------+--------------+-------------+-------+
| SEPAL LENGTH | SEPAL WIDTH | PETAL LENGTH | PETAL WIDTH | CLASS |
+--------------+-------------+--------------+-------------+-------+
|          6.4 |         2.8 |          5.6 |         2.2 |     2 |
|            5 |         2.3 |          3.3 |           1 |     1 |
+--------------+-------------+--------------+-------------+-------+
```

Then we can train a TensorFlow [DNNClassifier](https://www.tensorflow.org/api_docs/python/tf/estimator/DNNClassifier) model using the following statement.

```sql
SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes=3, model.hidden_units=[128,64],
    validation.select="SELECT * FROM iris.test"
LABEL class
INTO sqlflow_models.my_dnn_model;
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

|             Option                      | Environment Variable   | Description |
|-----------------------------------------|------------------------|-------------|
| -sqlflow_server \<quoted-query-string\> |     SQLFLOW_SERVER     | Specify sqlflow server address, in `host:port` form, e.g. `-sqlflow_server "localhost:50051"` |
| -e \<quoted-query-string\>              |                        | Execute from command line without entering interactive mode. e.g. <br>`-e "SELECT * FROM iris.train TRAIN DNNClassifier..." `<br>does the same thing as the training example above.|
| -f \<filename\>                         |                        | Execute from file without entering interactive mode. e.g. <br>`-f ./my_sqlflow.sql`<br>does the same thing as<br>`< ./my_sqlflow.sql` and `cat ./my_sqlflow.sql \| sqlflow`. The special file `-` means read from standard input. |
| -datasource \<database-connection-url\> |   SQLFLOW_DATASOURCE   | Connect to the specified database. e.g. `-datasource "mysql://root:root@tcp(localhost:3306)/" ` |
| -A                                      |                        | No auto completion for `sqlflow_models`. This gives a quicker start. |

## Environment Variable Config File Setup
You can specify some of the options in a config file named `.sqlflow_env` under your home directory.  This process is **optional** but can be convenient if you use the same config intensively.  The content are exported as environment variables at run time. Be aware the config file is just a default setting, you can overwrite them via corresponding command-line options.  Currently supported variables are listed in `Environment Variable` column in above table. You can setup the file using the following `bash` code.  
```bash
cat <<EOF >~/.sqlflow_env
SQLFLOW_SERVER=localhost:50051
SQLFLOW_DATASOURCE=mysql://root:root@tcp(localhost:3306)/?maxAllowedPacket=0
EOF
```

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
