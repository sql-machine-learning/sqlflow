# NOTE: The docker build context directory must be the root of the source tree.
# NOTE: To build the release image, SQLFlow must be built into ./build
FROM ubuntu:18.04

# Choose faster mirrors for apt-get and pip
COPY docker/dev/find_fastest_resources.sh /usr/local/bin/
RUN /bin/bash -c 'source find_fastest_resources.sh \
        && echo "Choose the fastest APT source ..." \
        && choose_fastest_apt_source \
        && echo "Choose the fastest PIP source ..." \
        && choose_fastest_pip_source'

# Install Python and JRE.  SQLFlow server requires Couler/Fluid to generate YAML
# files, and Couler/Fluid depends on Python.  The external parsers are in Java.
RUN apt-get -qq install -y \
           python3-dev python3-pip openjdk-8-jre-headless > /dev/null \
        && rm -rf /var/lib/apt/lists/* \
        && apt-get -qq clean -y
RUN ln -s /usr/bin/python3 /usr/local/bin/python
RUN ln -s /usr/bin/pip3 /usr/local/bin/pip

# Install the SQLFlow Python source code, which includes template code.
COPY python /usr/local/sqlflow/python
ENV PYTHONPATH=/usr/local/sqlflow/python:$PYTHONPATH

# Install pre-built SQLFlow components.
COPY build /build
ENV SQLFLOW_PARSER_SERVER_PORT=12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH="/usr/local/sqlflow/java"
RUN python3 -m pip install --quiet /build/couler*.whl /build/fluid*.whl \
        && mv /build/sqlflowserver /build/sqlflow /build/step /usr/local/bin/ \
        && mkdir -p $SQLFLOW_PARSER_SERVER_LOADING_PATH \
        && mv /build/*.jar $SQLFLOW_PARSER_SERVER_LOADING_PATH

# Install kubectl for submitting argo workflow
RUN apt-get -qq update && apt-get -qq install -y axel sudo
COPY scripts/travis/export_k8s_vars.sh /usr/local/bin/
COPY scripts/travis/install_kubectl.sh /usr/local/bin/
RUN bin/bash -c 'source export_k8s_vars.sh && install_kubectl.sh'

# Expose SQLFlow gRPC server and Jupyter Notebook server port.
EXPOSE 50051

# The sqlflowserver will find and launch external parser gRPC servers in Java
# according to environment variables SQLFLOW_PARSER_SERVER_PORT and
# SQLFLOW_PARSER_SERVER_LOADING_PATH.
CMD ["sqlflowserver"]
