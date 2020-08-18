# The CI image needs to build Go and Java tests and run Python tests,
# so it must contain the bulding tools.
#
# NOTE: The docker build context directory must be the root of the source tree.
# NOTE: To build the release image, SQLFlow must be built into ./build
FROM sqlflow:dev

RUN apt-get -qq update

COPY docker/ci/install-pips.bash /ci/
RUN /ci/install-pips.bash

COPY docker/ci/install-mysql-client.bash /ci/
RUN /ci/install-mysql-client.bash

COPY docker/ci/install-odps.bash /ci/
RUN /ci/install-odps.bash

ENV HADOOP_VERSION 3.2.1
ENV PATH /opt/hadoop-${HADOOP_VERSION}/bin:/usr/local/go/bin:/go/bin:$PATH
COPY docker/ci/install-hadoop.bash /ci/
RUN /ci/install-hadoop.bash

# scripts/test/workflow require kubectl.
COPY docker/ci/install-kubectl.bash /ci/
RUN /ci/install-kubectl.bash

# scripts/test/workflow require Docker.
RUN apt-get -qq install -y docker.io sudo > /dev/null

# Install the Python source code.
COPY python /usr/local/sqlflow/python
ENV PYTHONPATH=/usr/local/sqlflow/python:$PYTHONPATH

# Install pre-built SQLFlow components.
COPY build /build
ENV SQLFLOW_PARSER_SERVER_PORT=12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH="/usr/local/sqlflow/java"
RUN pip install --quiet /build/*.whl \
        && mv /build/sqlflowserver /build/sqlflow /build/step /usr/local/bin/ \
        && mkdir -p $SQLFLOW_PARSER_SERVER_LOADING_PATH \
        && mv /build/*.jar $SQLFLOW_PARSER_SERVER_LOADING_PATH

# Install Jupyter Notebook
COPY /docker/jupyter/js /jupyter/js/
COPY build/tutorial /workspace
COPY docker/ci/install-jupyter-ci.sh /jupyter/install-jupyter-ci.sh
RUN /bin/sh /jupyter/install-jupyter-ci.sh
ARG SQLFLOW_SERVER="localhost:50051"
ENV SQLFLOW_SERVER=${SQLFLOW_SERVER}
ARG SQLFLOW_DATASOURCE="mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
ENV SQLFLOW_DATASOURCE=${SQLFLOW_DATASOURCE}

WORKDIR /workspace
EXPOSE 8888

# Install MySQL and sample datasets.
RUN apt-get install -y mysql mysql-client
ARG MYSQL_PORT="3306"
ENV MYSQL_PORT=$MYSQL_PORT
EXPOSE $MYSQL_PORT

# Expose SQLFlow gRPC server port.
EXPOSE 50051 

ADD docker/ci/start.sh /
CMD ["bash", "/start.sh"]
