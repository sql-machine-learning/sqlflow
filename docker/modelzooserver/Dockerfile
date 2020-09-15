# NOTE: The docker build context directory must be the root of the source tree.
# NOTE: To build the release image, SQLFlow must be built into ./build
FROM alpine:3.12

ARG FIND_FASTED_MIRROR=true

# Choose faster mirrors for alpine and pip
# Install docker.io to release model defininiation
COPY docker/dev/find_fastest_resources.sh /usr/local/bin/
RUN /bin/sh -c 'if [ "$FIND_FASTED_MIRROR" == "true" ]; then source find_fastest_resources.sh && \
    echo "Choose the fastest Alpine source ..." && \
    choose_fastest_alpine_source && \
    echo "Choose the fastest PIP source ..." && \
    choose_fastest_pip_source; fi && \
    apk add --no-cache python3 py3-pip sudo bash docker-cli && \
    wget -q -O /etc/apk/keys/sgerrand.rsa.pub http://cdn.sqlflow.tech/alpine/sgerrand.rsa.pub.txt && \
    wget -q http://cdn.sqlflow.tech/alpine/glibc-2.31-r0.apk && \
    apk add glibc-2.31-r0.apk && \
    rm glibc-2.31-r0.apk && \
    ln -s /usr/bin/python3 /usr/local/bin/python && \
    ln -s /usr/bin/pip3 /usr/local/bin/pi'

# Install pre-built SQLFlow components.
COPY build/modelzooserver /usr/local/bin/modelzooserver

ARG MYSQL_ADDR="mysql://root:root@tcp(127.0.0.1:3306)/?"
ENV MYSQL_ADDR=${MYSQL_ADDR}

ARG SQLFLOW_MODEL_ZOO_REGISTRY_USER=""
ENV SQLFLOW_MODEL_ZOO_REGISTRY_USER=${SQLFLOW_MODEL_ZOO_REGISTRY_USER}

ARG SQLFLOW_MODEL_ZOO_REGISTRY_PASS=""
ENV SQLFLOW_MODEL_ZOO_REGISTRY_PASS=${SQLFLOW_MODEL_ZOO_REGISTRY_PASS}


# Expose SQLFLow Model Zoo server port.
EXPOSE 50055
VOLUME "/var/run/docker.sock"

CMD ["modelzooserver", "--mysql-addr", "${MYSQL_ADDR}"]
