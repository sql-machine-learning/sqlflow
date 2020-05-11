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
RUN apt-get -qq update \
        && /ci/install-build-essential.bash \
        && /ci/install-python.bash \
        && /ci/install-jupyter.bash \
        && /ci/install-mysql.bash \
        && /ci/install-odps.bash \
        && /ci/install-java.bash \
        && /ci/install-hadoop.bash

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

# Install Couler, Fluid, and model zoo.
COPY build/*.whl /usr/local/sqlflow/python/
RUN pip install /usr/local/sqlflow/python/*.whl

# Install the pre-built binaries
COPY build/sqlflowserver build/sqlflow build/step /usr/local/bin/

# Install the Java gRPC parser servers.
COPY build/*.jar /usr/local/sqlflow/java/
ENV SQLFLOW_PARSER_SERVER_PORT 12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH /usr/local/sqlflow/java

# Install the tutorials
COPY build/tutorial /workspace

 # Expose MySQL server, SQLFlow gRPC server, and Jupyter Notebook server port
EXPOSE 3306 50051 8888

ADD scripts/start.sh /
CMD ["bash", "/start.sh"]
