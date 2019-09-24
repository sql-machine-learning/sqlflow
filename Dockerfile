FROM ubuntu:16.04

# The default apt-get source archive.ubuntu.com might take too much traffic and
# has been slow. The following source makes docker build running faster.
RUN echo '\n\
deb http://us.archive.ubuntu.com/ubuntu/ xenial main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ xenial-security main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ xenial-updates main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ xenial-proposed main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ xenial-backports main restricted universe multiverse \n\
' > /etc/apt/sources.list

RUN apt-get update && apt-get install -y curl bzip2 \
    build-essential unzip sqlite3 libsqlite3-dev wget unzip git \
    openjdk-8-jdk maven libmysqlclient-dev

# Need Java SDK to build remote parsers.
ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64

# Miniconda - Python 3.6, 64-bit, x86, latest
ARG CONDA_ADD_PACKAGES=""
ARG PIP_ADD_PACKAGES=""
ARG TENSORFLOW_VERSION="2.0.0b1"
ARG WITH_SQLFLOW_MODELS="ON"

ENV GOPATH /go
ENV HADOOP_VERSION 3.2.1
ENV PATH /opt/hadoop-${HADOOP_VERSION}/bin:/miniconda/envs/sqlflow-dev/bin:/miniconda/bin:/usr/local/go/bin:/go/bin:$PATH
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/

# Main steps to Build
COPY . ${GOPATH}/src/sqlflow.org/sqlflow
RUN bash ${GOPATH}/src/sqlflow.org/sqlflow/scripts/build_docker_image.sh && \
    mkdir -p /workspace && \
    bash ${GOPATH}/src/sqlflow.org/sqlflow/scripts/convert_markdown_into_ipynb.sh && \
    rm -rf ${GOPATH}/src && rm -rf ${GOPATH}/bin
VOLUME /var/lib/mysql

# Prepare sample datasets
COPY doc/datasets/popularize_churn.sql \
     doc/datasets/popularize_iris.sql \
     doc/datasets/popularize_boston.sql \
     doc/datasets/popularize_creditcardfraud.sql \
     doc/datasets/create_model_db.sql \
     /docker-entrypoint-initdb.d/

ADD scripts/start.sh /
CMD ["bash", "/start.sh"]
