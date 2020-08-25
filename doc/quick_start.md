# Quick Start

It's quite simple to try out SQLFlow using [Docker](https://docs.docker.com/).

1. Install [Docker Community Edition](https://docs.docker.com/install/).
1. Run SQLflow by typing the below command:
   ```bash
   docker run --name=sqlflow-mysql -d -p 8888:8888 sqlflow/sqlflow:mysql
   docker run --net=container:sqlflow-mysql -d sqlflow/sqlflow:latest sqlflowserver
   docker run --net=container:sqlflow-mysql -d sqlflow/sqlflow:jupyter
   ```
1. Wait until the docker containers are all running, then access http://localhost:8888 in your Web browser.
1. Open the a tutorial notebook like `iris-dnn.ipynb` and run the examples.


For advanced usage, you might want to go on reading

- [Language Guide](language_guide.md)
- [Run Locally with Docker](run/docker.md)
- [Run on Google Cloud](run/gcp.md)
- [Run on Kubernetes](run/kubernetes.md)
- [Run with command-line tool](run/cli.md)
