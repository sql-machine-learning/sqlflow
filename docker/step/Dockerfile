FROM ubuntu:18.04 as base

# This script assume we are at sqlflow root directory and the directory is already built by sqlflow:dev
# The default source archive.ubuntu.com is busy and slow. We use the following source makes docker build running faster.
COPY docker/dev/find_fastest_resources.sh /usr/local/bin/find_fastest_resources.sh

ENV DEBIAN_FRONTEND=noninteractive
RUN /bin/bash -c 'source find_fastest_resources.sh \
  && echo "Choose the fastest APT source ..." \
  && choose_fastest_apt_source \
  && echo "Choose the fastest PIP source ..." \
  && choose_fastest_pip_source' && \
  apt-get update && \
  apt-get -qq install -y --no-install-recommends openjdk-8-jre-headless python3 libmysqlclient20 python3-idna libgomp1 python3-setuptools && \
  ln -sf /usr/share/zoneinfo/Etc/UTC /etc/localtime && \
  apt-get install -y tzdata > /dev/null && \
  dpkg-reconfigure --frontend noninteractive tzdata && \
  ln -s /usr/bin/python3 /usr/bin/python

# Build python wheels in sub stage so we can 
# keep the outcome and discard the build tool-chain
FROM base as builder
RUN mkdir /install
WORKDIR /install

ENV PATH="${PATH}:/install/bin"
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get -qq install -y wget unzip build-essential libmysqlclient-dev python3-pip && \
  ln -s /usr/bin/pip3 /usr/bin/pip && \
  pip install --upgrade pip && \
  wget -q http://docs-aliyun.cn-hangzhou.oss.aliyun-inc.com/assets/attach/119096/cn_zh/1557995455961/odpscmd_public.zip && \
  mkdir -p /install/local/odpscmd && \
  unzip -qq odpscmd_public.zip -d /install/local/odpscmd && \
  rm -rf odpscmd_public.zip

ADD build/sqlflow_models-0.0.1-py3-none-any.whl /
RUN bash -c 'pip install --no-cache-dir --prefix=/install \
    /sqlflow_models-0.0.1-py3-none-any.whl \
    six==1.15.0 \
    mysqlclient==1.4.4 \
    impyla==0.16.0 \
    pyodps==0.8.3 \
    oss2==2.9.0 \
    xgboost==0.90 \
    plotille==3.7 \
    seaborn==0.9.0 \
    dill==0.3.0 \
    sklearn2pmml==0.56.0 \
    shap==0.30.1'

RUN py3clean /install /usr/lib/python3.6

# Copy last stage's output, mostly python libs, to /usr
FROM base
COPY --from=builder /install /usr/

ADD build/step /usr/bin/
ADD build/*.jar /opt/sqlflow/parser/
ADD python/runtime /opt/sqlflow/python/runtime/
ADD python/symbol_extractor.py /opt/sqlflow/python/
ADD python/plotille_text_backend.py /opt/sqlflow/python/


ENV PATH "${PATH}:/usr/local/odpscmd/bin"
ENV PYTHONPATH "${PYTHONPATH}:/usr/lib/python3.6/site-packages:/opt/sqlflow/python"

