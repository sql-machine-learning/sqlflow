FROM ubuntu:18.04

# The default Ubuntu apt-get source archive.ubuntu.com is usually busy
# and slow.  If you are in the U.S., you might want to use
# http://us.archive.ubuntu.com/ubuntu/, or if you are in China, you
# can try https://mirrors.tuna.tsinghua.edu.cn/ubuntu/
ARG APT_MIRROR="http://us.archive.ubuntu.com/ubuntu/"
RUN echo "\n\
deb $APT_MIRROR bionic main restricted universe multiverse \n\
deb $APT_MIRROR bionic-security main restricted universe multiverse \n\
deb $APT_MIRROR bionic-updates main restricted universe multiverse \n\
deb $APT_MIRROR bionic-proposed main restricted universe multiverse \n\
deb $APT_MIRROR bionic-backports main restricted universe multiverse \n\
" > /etc/apt/sources.list

# Install dependencies.
COPY docker/ci/js /ci/js
# Required by install-jupyter.bash
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/

RUN apt-get -qq update

COPY docker/ci/install-build-essential.bash /ci/
RUN /ci/install-build-essential.bash

COPY docker/ci/install-python.bash /ci/
RUN /ci/install-python.bash

COPY docker/ci/install-pips.bash /ci/
RUN /ci/install-pips.bash

COPY docker/ci/install-jupyter.bash /ci/
RUN /ci/install-jupyter.bash

COPY docker/ci/install-mysql.bash /ci/
RUN /ci/install-mysql.bash

COPY docker/ci/install-odps.bash /ci/
RUN /ci/install-odps.bash

COPY docker/ci/install-java.bash /ci/
RUN /ci/install-java.bash

ENV HADOOP_VERSION 3.2.1
ENV PATH /opt/hadoop-${HADOOP_VERSION}/bin:/usr/local/go/bin:/go/bin:$PATH
COPY docker/ci/install-hadoop.bash /ci/
RUN /ci/install-hadoop.bash

# Install sample datasets for CI and demo.
COPY doc/datasets/popularize_churn.sql \
     doc/datasets/popularize_iris.sql \
     doc/datasets/popularize_boston.sql \
     doc/datasets/popularize_creditcardfraud.sql \
     doc/datasets/popularize_imdb.sql \
     doc/datasets/create_model_db.sql \
     /docker-entrypoint-initdb.d/
VOLUME /var/lib/mysql

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
        && mv /build/*.jar $SQLFLOW_PARSER_SERVER_LOADING_PATH \
        && mv /build/tutorial /workspace

# Expose MySQL server, SQLFlow gRPC server, and Jupyter Notebook server port.
EXPOSE 3306 50051 8888

ADD scripts/start.sh /
CMD ["bash", "/start.sh"]
