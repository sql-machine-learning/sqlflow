# Contribute Models

In this document, we'll describe the steps to follow when contributing models to SQLFlow.

## Prepare Model Development Git Repository

1. You can contribute to SQLFlow's [model zoo repo](https://github.com/sql-machine-learning/models) by:
    1. Fork SQLFlow's model zoo repo: click "Fork" button on the right corner on page https://github.com/sql-machine-learning/models .
    1. Clone your forked repo by `git clone [your forked repo url]`, you can find the forked repo URL by clicking the green button "Clone or download".
    1. Move to the cloned directory: `cd models`.
1. Or you can create a new git repository to store your model code:
    1. Create a new repository on [github](https://github.com) or any other git systems.
    1. Move to the directory of the repository: `cd my_models` (assume you created a repo named "my_models").
    1. Create a directory under `my_models` to store Python package: `mkdir my_awesome_model`.

## Start a Docker Container as the Develop Environment

```bash
docker run -p 8888:8888 -v $PWD/my_awesome_model:/workspace/my_awesome_model  sqlflow/sqlflow bash -c 'export PYTHONPATH=/workspace:$PYTHONPATH; bash /start.sh'
```

Note that we set the environment variable `PYTHONPATH` so that we can directly test out the model inside this container. Change the directory to `sqlflow_models` if you are contributing models to https://github.com/sql-machine-learning/models.

## Develop In the Jupyter Notebook

Open the browser and go to http://localhost:8888, it's a Jupyter notebook environment, you can see your model development directory `my_awesome_model` together with SQLFlow's basic tutorials.

<p align="center">
<img src="figures/jupyter_develop.jpg">
</p>

Click into the directory `my_awesome_model` and add the `__init__.py` and your new model file, e.g. `mydnnclassifier.py`. In `__init__.py` you should expose your model classes by adding lines like `from mydnnclassifier import MyAwesomeClassifier`.

Write model code following instructions: https://github.com/sql-machine-learning/models/blob/develop/doc/contribute_models.md, or copy the sample code from https://github.com/sql-machine-learning/models/blob/develop/sqlflow_models/dnnclassifier.py.

## Testing and Debugging

Go back to http://localhost:8888, add an `ipynb` file to test the model by clicking the button "New" -> "Python 3"

<p align="center">
<img src="figures/jupyter_create_ipynb.jpg">
</p>

Write an SQLFlow statement to test the model using iris dataset (you need to import the dataset to MySQL if you want to test the model using other datasets.), assume you have developed a model class name:

```sql
%%sqlflow
SELECT * FROM iris.train
TO TRAIN my_awesome_model.MyAwesomeClassifier
WITH model.n_classes=3
LABEL class
INTO models_db.awesome_model;
```

you may go back to `mydnnclassifier.py` and modify the model code until it works as you expected.

## Publish Your Model

In the final step, you need to publish your model so that other SQLFlow users can get the model and use it.

1. If you are contributing to https://github.com/sql-machine-learning/models, file a pull request on Github to merge your code to SQLFlow's models repo. The model should be available when SQLFlow's Docker image `sqlflow/sqlflow` is updated.
1. If you are creating your own repo, you need to write a `Dockerfile` to build your model into a Docker image:
    1. Write a `Dockerfile` like below:
    ```docker
    FROM sqlflow/sqlflow
    ADD my_awesome_model/ /models/
    ```
    1. Then build and push the Docker image by:
    ```
    docker build -t your-registry.com/model_image .
    docker push your-registry.com/model_image
    ```
    1. Then use the model image in SQLFlow by adding the Docker image name before the model name:
    ```sql
    SELECT * FROM iris.train
    TO TRAIN your-registry.com/model_image/MyAwesomeClassifier
    WITH model.n_classes=3
    LABEL class
    INTO models_db.awesome_model;
    ```
