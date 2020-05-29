# The CI image needs to build Go and Java tests and run Python tests,
# so it must contain the bulding tools.
#
# NOTE: The docker build context directory must be the root of the source tree.
# NOTE: To build the release image, SQLFlow must be built into ./build
FROM sqlflow:dev

# Install dependencies.
COPY docker/ci/js /ci/js
# Required by install-jupyter.bash
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/

RUN apt-get -qq update

COPY docker/ci/install-pips.bash /ci/
RUN /ci/install-pips.bash

COPY docker/ci/install-jupyter.bash /ci/
RUN /ci/install-jupyter.bash

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
RUN mkdir -p "$SQLFLOW_PARSER_SERVER_LOADING_PATH" \
	&& mv /build/*.jar "$SQLFLOW_PARSER_SERVER_LOADING_PATH" \
	&& mv /build/sqlflowserver /build/sqlflow /build/step /usr/local/bin/ \
	&& pip install --quiet /build/*.whl \
	&& mv /build/tutorial /workspace
WORKDIR /workspace


# Expose SQLFlow gRPC server and Jupyter Notebook server port.
EXPOSE 50051 8888

ADD docker/ci/start.sh /
CMD ["bash", "/start.sh"]
