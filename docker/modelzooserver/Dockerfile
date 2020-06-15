# NOTE: The docker build context directory must be the root of the source tree.
# NOTE: To build the release image, SQLFlow must be built into ./build
FROM ubuntu:18.04

# Choose faster mirrors for apt-get and pip
# Install docker.io to release model defininiation
COPY docker/dev/find_fastest_resources.sh /usr/local/bin/
RUN bash -c 'source find_fastest_resources.sh && \
             echo "Choose the fastest APT source ..." && \
             choose_fastest_apt_source' && \
    apt-get -qq update && \
    apt-get -qq install -y axel sudo docker.io

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
