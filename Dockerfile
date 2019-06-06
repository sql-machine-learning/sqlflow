FROM ubuntu:16.04

RUN apt-get update && apt-get install -y curl bzip2 \
    build-essential unzip sqlite3 libsqlite3-dev wget git

# Miniconda - Python 3.6, 64-bit, x86, latest
ARG CONDA_ADD_PACKAGES=""
ARG PIP_ADD_PACKAGES=""

ENV GOPATH /go
ENV PATH /miniconda/bin:/miniconda/envs/sqlflow-dev/bin:/usr/local/go/bin:/go/bin:$PATH
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/

# Main Steps to Build
COPY scripts/image_build.sh /image_build.sh
RUN bash /image_build.sh && rm -f /image_build.sh
VOLUME /var/lib/mysql

# Prepare sample datasets
COPY example/datasets/popularize_churn.sql example/datasets/popularize_iris.sql example/datasets/create_model_db.sql /docker-entrypoint-initdb.d/

ADD scripts/start.sh /
CMD ["bash", "/start.sh"]
