FROM ubuntu:16.04

# The default source archive.ubuntu.com is busy and slow. We use the following source makes docker build running faster.
RUN echo '\n\
deb http://cn.archive.ubuntu.com/ubuntu/ bionic main restricted universe multiverse \n\
deb http://cn.archive.ubuntu.com/ubuntu/ bionic-security main restricted universe multiverse \n\
deb http://cn.archive.ubuntu.com/ubuntu/ bionic-updates main restricted universe multiverse \n\
deb http://cn.archive.ubuntu.com/ubuntu/ bionic-proposed main restricted universe multiverse \n\
deb http://cn.archive.ubuntu.com/ubuntu/ bionic-backports main restricted universe multiverse \n\
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

RUN curl -sL https://repo.continuum.io/miniconda/Miniconda3-latest-Linux-x86_64.sh -o mconda-install.sh && \
bash -x mconda-install.sh -b -p miniconda && \
rm mconda-install.sh

RUN /miniconda/bin/conda create -y -q -n sqlflow-dev python=3.6 && \
echo ". /miniconda/etc/profile.d/conda.sh" >> ~/.bashrc && \
echo "source activate sqlflow-dev" >> ~/.bashrc

RUN apt-get install -y libmysqlclient-dev

RUN bash -c "source /miniconda/bin/activate sqlflow-dev && python -m pip install numpy==1.16.1 tensorflow==2.0.0b1 mysqlclient==1.4.4 impyla==0.16.0 pyodps==0.8.3"
ENV PATH /miniconda/envs/sqlflow-dev/bin:/miniconda/bin:$PATH
ADD sqlflow_submitter /miniconda/envs/sqlflow-dev/lib/python3.6/site-packages/sqlflow_submitter
