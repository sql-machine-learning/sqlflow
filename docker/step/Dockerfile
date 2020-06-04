FROM ubuntu:18.04

# This script assume we are at sqlflow root directory and the directory is already built by sqlflow:dev
# The default source archive.ubuntu.com is busy and slow. We use the following source makes docker build running faster.
COPY docker/dev/find_fastest_resources.sh /usr/local/bin/find_fastest_resources.sh

RUN /bin/bash -c 'source find_fastest_resources.sh \
  && echo "Choose the fastest APT source ..." \
  && choose_fastest_apt_source \
  && echo "Choose the fastest PIP source ..." \
  && choose_fastest_pip_source'

RUN apt-get install -y openjdk-8-jre-headless wget unzip > /dev/null

RUN wget -q http://docs-aliyun.cn-hangzhou.oss.aliyun-inc.com/assets/attach/119096/cn_zh/1557995455961/odpscmd_public.zip && \
unzip -qq odpscmd_public.zip -d /usr/local/odpscmd && \
ln -s /usr/local/odpscmd/bin/odpscmd /usr/local/bin/odpscmd && \
rm -rf odpscmd_public.zip

ADD build/step /usr/bin/
ADD build/*.jar /opt/sqlflow/parser/

ENV DEBIAN_FRONTEND=noninteractive
RUN ln -sf /usr/share/zoneinfo/Etc/UTC /etc/localtime && \
apt-get install -y tzdata > /dev/null && \
dpkg-reconfigure --frontend noninteractive tzdata

RUN apt-get install -y build-essential libmysqlclient-dev > /dev/null

RUN apt-get install -y python3 python3-pip
RUN ln -s /usr/bin/python3 /usr/bin/python
RUN ln -s /usr/bin/pip3 /usr/bin/pip

RUN bash -c "pip install numpy==1.16.1 \
    tensorflow==2.0.0b1 \
    mysqlclient==1.4.4 \
    impyla==0.16.0 \
    pyodps==0.8.3 \
    oss2==2.9.0 \
    xgboost==0.90 \
    shap==0.30.1 \
    plotille==3.7 \
    seaborn==0.9.0 \
    dill==0.3.0"

ADD python/sqlflow_submitter /opt/sqlflow/python/sqlflow_submitter/
ADD build/models/sqlflow_models /opt/sqlflow/python/sqlflow_models/

ENV PYTHONPATH /opt/sqlflow/python

