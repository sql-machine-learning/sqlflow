# Installation

SQLFlow is currently under active development. For those who are interested in trying
it out, we have provided the instructions and demo. Play around with it. Any bug report and
issue is welcome. :)


## Preparation

1. Install [Docker Community Edition](https://docs.docker.com/install/) on your Macbook.
1. Build and run a dockerized MySQL server (In this example, called sqlflowdata) following [example/datasets/README.md](/example/datasets/README.md). Note that there is no need to install a local MySQL server, in which case you will have a port conflict in 3306. 
1. Pull the latest SQLFlow Docker image: `docker pull sqlflow/sqlflow:latest`.

## Running your first SQLFlow query

After data is popularized in MySQL, let's test the installation from running a query in Jupyter Notebook:

1. Start a Docker container that runs SQLFlow server and Jupyter Notebook. If you are
   using Docker for Linux, please change `host.docker.internal:3306` to `localhost:3306`.

   ```
   docker run --rm -it -p 8888:8888 sqlflow/sqlflow:latest \
   bash -c "sqlflowserver --db_user root --db_password root --db_address host.docker.internal:3306 &
   SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root"
   ```

   If you are using Docker for Mac, please be aware the option `--db_address host.docker.internal:3306` where
   `host.docker.internal` translates to the host ip address as recommended [here](https://docs.docker.com/docker-for-mac/networking/).

   If you are running MySQL on remote, please be aware that MySQL only allows connections from localhost
   by default. Fix can be found [here](https://stackoverflow.com/questions/14779104/how-to-allow-remote-connection-to-mysql).

1. Open a web browser, go to `localhost:8888` and paste the token output from Notebook command above. In a Notebook cell, you should be able to test a select statement to fetch 5 records from train table in Iris database. 

   ```
   %%sqlflow
   select * from iris.train limit 5;
   ```

1. Feel free to explore more examples at [example.ipynb](/example/jupyter/example.ipynb) if you are new to Jupyter Notebook.
