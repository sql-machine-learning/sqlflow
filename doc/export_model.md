# Export Trained Models

Once we train a model using SQLFlow via a statement like the following,

```sql
SELECT * FROM iris.train
TO TRAIN DNNClassifier
WITH model.n_classes=3,model.hidden_units=[10,20]
INTO sqlflow_models.my_dnn_model;
```

the trained model `sqlflow_models.my_dnn_model` is saved in the database.
Anyone with the read access can write SQLFlow statement to visually explain
the model or to use the model for prediction.

In some cases, you might want to export and download a trained model from the
database, so you can use it out of SQLFlow, for example, load it to an online
prediction service of an online advertising system. To export a model, you can
use the [command-line tool](run/cli.md) `sqlflow`.
we can download the trained model (`sqlflow_models.my_dnn_model`), using the
command:

```shell
# Configure database connection string
export SQLFLOW_DATASOURCE="mysql://root:root@tcp(127.0.0.1:3306)/?"
# Configure sqlflow server address
export SQLFLOW_SERVER="localhost:50051"
# Assume you have trained a model named sqlflow_models.my_dnn_model
./sqlflow get model "sqlflow_models.my_dnn_model"
```

If the model has been downloaded successfully, you can see the below output on the
terminal:

```
model "sqlflow_models.my_dnn_model" downloaded successfully at
/your/working/directory/model_dump.tar.gz
```

`model_dump.tar.gz` contains below files/folders:

- `exported_path` file contains one line indicating the path of the Tensorflow
  exported model (if the model is a Tensorflow model).
- `model_meta.json` file is a JSON serialized file containing how the model is
  trained by SQLFlow.
- `model_save` directory contains all the files of the trained model.
