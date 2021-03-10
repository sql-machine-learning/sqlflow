# NOTE: The docker build context directory must be the root of the source tree.
# NOTE: To build the release image, SQLFlow must be built into ./build
FROM alpine:3.12

ARG FIND_FASTED_MIRROR=true

# Choose faster mirrors for apt-get and pip
COPY docker/dev/find_fastest_resources.sh /usr/local/bin/
RUN /bin/sh -c 'if [ "$FIND_FASTED_MIRROR" == "true" ]; then source find_fastest_resources.sh && \
    echo "Choose the fastest Alpine source ..." && \
    choose_fastest_alpine_source && \
    echo "Choose the fastest PIP source ..." && \
    choose_fastest_pip_source; fi'

# Install Python and JRE.  SQLFlow server requires Couler/Fluid to generate YAML
# files, and Couler/Fluid depends on Python.  The external parsers are in Java.
RUN apk update \
    && apk add --no-cache python3 py3-pip openjdk8-jre-base axel sudo bash && \
    wget -q -O /etc/apk/keys/sgerrand.rsa.pub http://cdn.sqlflow.tech/alpine/sgerrand.rsa.pub.txt && \
    wget -q http://cdn.sqlflow.tech/alpine/glibc-2.31-r0.apk && \
    apk add glibc-2.31-r0.apk && \
    rm glibc-2.31-r0.apk && \
    ln -s /usr/bin/python3 /usr/local/bin/python && \
    ln -s /usr/bin/pip3 /usr/local/bin/pip

# Install the SQLFlow Python source code, which includes template code.
COPY python /usr/local/sqlflow/python
ENV PYTHONPATH=/usr/local/sqlflow/python:$PYTHONPATH

# Install pre-built SQLFlow components.
COPY build /build
ENV SQLFLOW_PARSER_SERVER_PORT=12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH="/usr/local/sqlflow/java"
RUN python3 -m pip install --quiet /build/couler*.whl /build/fluid*.whl && \
        mv /build/sqlflowserver /build/sqlflow /build/step /usr/local/bin/ && \
        mkdir -p $SQLFLOW_PARSER_SERVER_LOADING_PATH && \
        mv /build/*.jar $SQLFLOW_PARSER_SERVER_LOADING_PATH && \
        rm -rf /build

# Install kubectl for submitting argo workflow
COPY scripts/travis/export_k8s_vars.sh /usr/local/bin/
COPY scripts/travis/install_kubectl.sh /usr/local/bin/
RUN bin/bash -c 'source export_k8s_vars.sh && install_kubectl.sh'

# Expose SQLFlow gRPC server and Jupyter Notebook server port.
EXPOSE 50051

# The sqlflowserver will find and launch external parser gRPC servers in Java
# according to environment variables SQLFLOW_PARSER_SERVER_PORT and
# SQLFLOW_PARSER_SERVER_LOADING_PATH.
CMD ["sqlflowserver"]
