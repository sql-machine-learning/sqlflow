# Extended SQL runner

## Overview

This package implements a extended SQL runner in Python, which is used to parse the `json` job description and runs the ML job using TF. The whole procedure includes

1. Parsing the `json` job description.
1. Retrieving the data from MySQL via [MySQL Connector Python API](https://dev.mysql.com/downloads/connector/python/). Optionally, retrieving the trained model from file system.
1. Either training the model or predicting using the trained model by calling the user specified TensorFlow estimator.
1. Writing the trained model or prediction results into a table.

