FROM ubuntu:16.04

RUN apt-get update && apt-get install -y curl bzip2 \
    build-essential unzip sqlite3 libsqlite3-dev

# Miniconda - Python 3.6, 64-bit, x86, latest
ARG CONDA_OS=Linux
ARG CONDA_ADD_PACKAGES=""
ARG PIP_ADD_PACKAGES=""

RUN curl -sL https://repo.continuum.io/miniconda/Miniconda3-latest-Linux-x86_64.sh -o mconda-install.sh && \
    bash -x mconda-install.sh -b -p miniconda && \
    rm mconda-install.sh && \
    /miniconda/bin/conda create -y -q -n sqlflow-dev python=3.6 ${CONDA_ADD_PACKAGES} && \
    echo ". /miniconda/etc/profile.d/conda.sh" >> ~/.bashrc && \
    echo "source activate sqlflow-dev" >> ~/.bashrc && \
    /bin/bash -c "source /miniconda/bin/activate sqlflow-dev && python -m pip install \
    tensorflow==2.0.0-alpha0 \
    mysql-connector-python \
    impyla \
    jupyter \
    sqlflow \
    pre-commit \
    ${PIP_ADD_PACKAGES} \
    "
ENV PATH="/miniconda/bin:$PATH"

# Install go 1.11.5
RUN apt-get install -y wget git && \
    wget --quiet https://dl.google.com/go/go1.11.5.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.11.5.linux-amd64.tar.gz && \
    rm go1.11.5.linux-amd64.tar.gz && \
    mkdir -p /go
ENV GOPATH /go
ENV PATH $PATH:/usr/local/go/bin:/go/bin

RUN go get github.com/golang/protobuf/protoc-gen-go && \
    mv /go/bin/protoc-gen-go /usr/local/bin/ && \
    go get golang.org/x/lint/golint

# Install protobuf compiler
RUN wget --quiet https://github.com/protocolbuffers/protobuf/releases/download/v3.6.1/protoc-3.6.1-linux-x86_64.zip && \
    unzip -qq protoc-3.6.1-linux-x86_64.zip -d /usr/local && \
    rm protoc-3.6.1-linux-x86_64.zip

# install mysql without a password prompt
RUN echo 'mysql-server mysql-server/root_password password root' | debconf-set-selections && \
    echo 'mysql-server mysql-server/root_password_again password root' | debconf-set-selections && \
    apt-get install -y mysql-server && \
    mkdir -p /var/run/mysqld && \
    mkdir -p /var/lib/mysql && \
    chown mysql:mysql /var/run/mysqld && \
    mkdir -p /docker-entrypoint-initdb.d
VOLUME /var/lib/mysql

# Add the python module of sqlflow to PYTHONPATH
ENV PYTHONPATH $PYTHONPATH:$GOPATH/src/github.com/sql-machine-learning/sqlflow/sql/python

# Build SQLFlow binaries by git clone the latest develop branch.
# During development, /go will be overridden by -v.
RUN mkdir -p /go/src/github.com/sql-machine-learning && \
    cd /go/src/github.com/sql-machine-learning && \
    git clone -q https://github.com/sql-machine-learning/sqlflow.git && \
    cd sqlflow && \
    go generate ./... && \
    go get -v -t ./... && \
    go install -v ./... && \
    cd /

# Install latest sqlflow_models for testing custom models, see main_test.go:CaseTrainCustomModel
RUN git clone https://github.com/sql-machine-learning/models.git && \
    cd models && \
    bash -c "source activate sqlflow-dev && python setup.py install" && \
    cd ..

# Fix jupyter server "connecting to kernel" problem
# https://github.com/jupyter/notebook/issues/2664#issuecomment-468954423
RUN /bin/bash -c "source activate sqlflow-dev && python -m pip install tornado==4.5.3"

# Load sqlflow Jupyter magic command automatically. c.f. https://stackoverflow.com/a/32683001.
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/
RUN mkdir -p $IPYTHON_STARTUP && \
    echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")' >> $IPYTHON_STARTUP/00-first.py && \
    echo 'get_ipython().magic(u"%autoreload 2")' >> $IPYTHON_STARTUP/00-first.py && \
    curl https://raw.githubusercontent.com/sql-machine-learning/sqlflow/develop/example/jupyter/example.ipynb --output /example.ipynb

# Make sqlflow-dev pyenv the default Python environment
ENV PATH=/miniconda/envs/sqlflow-dev/bin:$PATH

# Prepare sample datasets
COPY example/datasets/popularize_churn.sql /docker-entrypoint-initdb.d/popularize_churn.sql
COPY example/datasets/popularize_iris.sql /docker-entrypoint-initdb.d/popularize_iris.sql
COPY example/datasets/create_model_db.sql /docker-entrypoint-initdb.d/create_model_db.sql

ADD scripts/startall.sh /
CMD ["bash", "/startall.sh"]
