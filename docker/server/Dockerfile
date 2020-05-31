# NOTE: The docker build context directory must be the root of the source tree.
# NOTE: To build the release image, SQLFlow must be built into ./build
FROM ubuntu:18.04

# Install Python, because SQLFlow server requires Couler/Fluid to generate YAML
# files, and Couler/Fluid depends on Python.
RUN apt-get -qq install -y --no-install-recommends \
        python3-dev python3-pip > /dev/null

# Install the SQLFlow Python source code, which includes template code.
COPY python /usr/local/sqlflow/python
ENV PYTHONPATH=/usr/local/sqlflow/python:$PYTHONPATH

# Install Java runtime, because the external parsers are in Java.
RUN apt-get -qq update \
        && apt-get -qq install -y openjdk-8-jre-headless \
        rm -rf /var/lib/apt/lists/* \
        apt-get -qq clean -y

# Install pre-built SQLFlow components.
COPY build /build
ENV SQLFLOW_PARSER_SERVER_PORT=12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH="/usr/local/sqlflow/java"
RUN pip install --quiet /build/couler*.whl /build/fluid*.whl \
        && mv /build/sqlflowserver /build/sqlflow /build/step /usr/local/bin/ \
        && mkdir -p $SQLFLOW_PARSER_SERVER_LOADING_PATH \
        && mv /build/*.jar $SQLFLOW_PARSER_SERVER_LOADING_PATH

# Expose SQLFlow gRPC server and Jupyter Notebook server port.
EXPOSE 50051

# The sqlflowserver will find and launch external parser gRPC servers in Java
# according to environment variables SQLFLOW_PARSER_SERVER_PORT and
# SQLFLOW_PARSER_SERVER_LOADING_PATH.
CMD ["sqlflowserver"]
