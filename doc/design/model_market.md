# Model Market

The model market is a web site where model developers can publish and share their models and analysts can find some useful models to finish the analysis work. The model market can be deployed anywhere like on the cloud or on-premise with some configurations.

In the [model zoo design](model_zoo.md), we described how model developers develop, publish and share custom models on SQLFlow, and how analysts can make use of the shared model with SQLFlow.

Here, we'll describe how to build the model market model developers and analysts.

## Overview

The model market is designed to:

1. Manage user login and logout.
1. View published images available for the current user.
1. View published trained models available for the current user.
1. Publish (or remove) model definition images.
1. Publish (or remove) trained models.
1. Share (or remove share) model definition images to other users.
1. Share (or remove share) trained models to other users.

<p align="center">
<img src="figures/model_market_overview.png">
</p>

In order to support publishing and sharing models securely, the model market is able to communicate with some [SSO](https://en.wikipedia.org/wiki/Single_sign-on) service to authenticate users. Only users that are logged in can do operations on the model market.

Then model definition images and trained model can have below accessibility settings:

1. Private: Only visible to the current user.
2. Public: Readable by every user.
3. Private but shared to some users: visible to the current user and users that shared to.

## Steps to View Model Definitions and Trained Models

1. Login to model market.
1. Click at the "Model Definitions" tab to see the list of model definition Docker images and the model class names in each Docker image.
1. Click at "Trained Models" tab to see all trained models the current user have published by using SQLFlow `PUBLISH` statement, the evaluation result of the trained model will also be available.

## Steps to Publish a Docker Image

1. Login to model market.
1. Go to the "Model Definitions" tab.
1. Click "New" to add a new image.
1. Input the full Docker image address in the dialog and click "OK".
1. ***Optional***: The system will call the Docker registry API to grant access for SQLFlow to pull the image. When using a public image, this step will be skipped. If access can be granted, a message should be shown on the web page.
1. The system will start to run several checks and tests using the Docker image. If all the checks have passed, the image is added.

**NOTE: Docker image with different [tags](https://www.freecodecamp.org/news/an-introduction-to-docker-tags-9b5395636c2a/) will be recognized as different images.

## Steps to Share Model Definition Docker Images to Other Users

1. Login to model market.
1. Go to the "Model Definitions" tab.
1. Find a Docker image in the list to be shared and click on the button "share".
1. Input the user name or user ID to share to and click "OK".

The system will call Docker registry API to grant access for the user shared to after these operations. The model market will save the grant information in a database table `image_shares` that have below columns:

1. `OwnerID`: Docker image owner user ID.
1. `SharedUserID`: User ID that the image is shared to.

## Steps to Publish a Trained Model

The following SQL statement will publish a trained model named `my_first_model` to the model market.

```sql
SQLFLOW PUBLISH my_first_model
    [TO https://models.sqlflow.org/user_name]
```

By publishing a trained model, the model market will save the trained model weights together with ownership information into two database tables (can use a MySQL service in general): the **trained models table** and the **evaluation result table**.

### Trained Models Table

Once a training job completes, the submitter program adds/updates a row of the trained models' table, which contains (at least) the following fields.

1. The model ID (or model name), specified by the INTO clause, or `my_first_model` in the example at [model_zoo_design](model_zoo.md).
1. The creator, the current user ID.
1. The model zoo release, which is a Docker image commit ID, or `a_data_scientist/regressors` in the above example.
1. The model definition, which is a Python class name, or `DNNRegressor` in the above example.
1. The model weights file path, the path to the trained model parameters on the distributed filesystem of the cluster.

It is necessary to have the model ID so users can refer to the trained model when they want to use it.  Suppose that the user typed the prediction SQL statement using this model name. SQLFlow server will convert it into a submitter program and run it with the Docker image used to train the model. Therefore, the Docker image ID is also required. The hyperparameters and data converter can be loaded when loading the model weights, helps the prediction submitter to use the conversion rules consistent with the ones used when training.

### The Model Evaluation Table

When publishing a trained model, the evaluation result will be saved to the model evaluation table,
which contains the following fields:

1. model ID
1. evaluation dataset (select statement used to fetch evaluation dataset)
1. metrics

Different kinds of models might use various metrics, so the field metrics might be string-typed and saves a JSON, like

```json
{
   "recall": 0.45,
   "precision": 0.734
}
```

## Steps to Share Trained Models to Other Users

1. Login to model market.
1. Go to the "Trained Models" tab.
1. Find a trained model in the list to be shared and click on the button "share".
1. Input the user name or user ID to share to and click "OK".

The model market will save the grant information in a database table `traind_model_shares` that have below columns:

1. `OwnerID`: trained model owner user ID.
1. `SharedUserID`: User ID that the trained model is shared to.

When one user is trying to use the trained model in an SQLFlow statement, the `sqlflowserver` will first check whether the user is the owner of the trained model or the model has been shared with the user. Or else, `sqlflowserver` will return an error.

## Summarization

The model market can be deployed anywhere like on the cloud or on-premise with some configurations. Even some secret model development can be done by using the model market as a collaboration platform. Either model definitions and trained models are managed securely by SQLFlow and model market, people can get access to your model only if you share it with them.
