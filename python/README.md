# scheduler

## Overview

This package implements a job scheduler in Python, which is used to parse the `json` job description and schedules the ML job using TF. The whole procedure includes

1. Parsing the `json` job description.
1. Retrieving the data from MySQL via [MySQL Connector Python API](https://dev.mysql.com/downloads/connector/python/). Optionally, retrieving the model from MySQL.
1. Training the model or predicts using the trained model by calling the user specified TensorFlow estimator.
1. Writing the trained model or prediction results into a table.

