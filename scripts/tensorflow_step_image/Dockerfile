FROM ubuntu:16.04

# The default source archive.ubuntu.com is busy and slow. We use the following source makes docker build running faster.
RUN echo '\n\
deb http://cn.archive.ubuntu.com/ubuntu/ xenial main restricted universe multiverse \n\
deb http://cn.archive.ubuntu.com/ubuntu/ xenial-security main restricted universe multiverse \n\
deb http://cn.archive.ubuntu.com/ubuntu/ xenial-updates main restricted universe multiverse \n\
deb http://cn.archive.ubuntu.com/ubuntu/ xenial-proposed main restricted universe multiverse \n\
deb http://cn.archive.ubuntu.com/ubuntu/ xenial-backports main restricted universe multiverse \n\
' > /etc/apt/sources.list

RUN apt-get update && apt-get install -y openjdk-8-jdk maven
RUN apt-get install -y wget unzip

RUN wget -q http://docs-aliyun.cn-hangzhou.oss.aliyun-inc.com/assets/attach/119096/cn_zh/1557995455961/odpscmd_public.zip && \
unzip -qq odpscmd_public.zip -d /usr/local/odpscmd && \
ln -s /usr/local/odpscmd/bin/odpscmd /usr/local/bin/odpscmd && \
rm -rf odpscmd_public.zip

ADD repl /usr/bin/
ADD parser-1.0-SNAPSHOT-jar-with-dependencies.jar /opt/sqlflow/parser/

ENV DEBIAN_FRONTEND=noninteractive
RUN ln -sf /usr/share/zoneinfo/Etc/UTC /etc/localtime && \
apt-get install -y tzdata && \
dpkg-reconfigure --frontend noninteractive tzdata

# install python
RUN apt-get install -y build-essential
RUN apt-get install -y curl

RUN apt-get install -y libmysqlclient-dev

RUN apt-get install python3 python3-pip
RUN ln -s /usr/bin/python3 /usr/bin/python
RUN ln -s /usr/bin/pip3 /usr/bin/pip

RUN bash -c "pip install numpy==1.16.1 tensorflow==2.0.0b1 mysqlclient==1.4.4 impyla==0.16.0 pyodps==0.8.3"
ADD sqlflow_submitter $SQLFLOWPATH/python/
