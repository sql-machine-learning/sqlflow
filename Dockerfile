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
COPY docker/ci /ci
# Required by install-jupyter.bash
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/
RUN apt-get -qq update
RUN /ci/install-build-essential.bash
RUN /ci/install-python.bash
RUN /ci/install-pips.bash
RUN /ci/install-jupyter.bash
RUN /ci/install-mysql.bash
RUN /ci/install-odps.bash
RUN /ci/install-java.bash
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
        && mv /build/*.jar $SQLFLOW_PARSER_SERVER_LOADING_PATH \
        && mv /build/tutorial /workspace

# Expose MySQL server, SQLFlow gRPC server, and Jupyter Notebook server port.
EXPOSE 3306 50051 8888

ADD scripts/start.sh /
CMD ["bash", "/start.sh"]
