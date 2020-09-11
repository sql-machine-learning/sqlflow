# Download Your Trained Model

Once you have trained your model with SQLFlow you may need to download your
model so that you can deploy some online service to do real-time prediction.

To download your trained model from SQLFlow, you need to install our
[command-line tool](run/cli.md) first. Then run the below command to download
your model:

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
