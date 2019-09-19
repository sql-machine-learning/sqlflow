# Quick Start

It's quite simple to try out SQLFlow using [Docker](https://docs.docker.com/).

1. Install [Docker Community Edition](https://docs.docker.com/install/) on your PC/Macbook/Server
   if it's not installed yet.
1. Pull the latest SQLFlow Docker image: `docker pull sqlflow/sqlflow`.
1. Start a docker container running SQLflow services: `docker run -it -p 8888:8888 sqlflow/sqlflow`.
1. Open a web browser, go to `localhost:8888`, open `tutorial_dnn_iris.ipynb` file, then you can
   follow the tutorial and run the SQL statements to run the training and prediction.


For advanced usage, you can continue reading:

[User Guide](user_guide.md)

[Run SQLFlow Command Line](demo.md)

[Docker Installation](docker_install.md)
