FROM ubuntu:18.04

# The default source archive.ubuntu.com is busy and slow. We use the following source makes docker build running faster.
RUN echo '\n\
deb http://us.archive.ubuntu.com/ubuntu/ bionic main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ bionic-security main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ bionic-updates main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ bionic-proposed main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ bionic-backports main restricted universe multiverse \n\
' > /etc/apt/sources.list

RUN apt-get -qq update

# Install wget, curl, unzip, bzip2, git
COPY scripts/docker/install-download-tools.bash /
RUN /install-download-tools.bash

# Install shellcheck
COPY scripts/docker/install-shell-tools.bash /
RUN /install-shell-tools.bash

# MySQL server and client
COPY scripts/docker/install-mysql.bash /
RUN /install-mysql.bash

# Install protobuf and protoc
COPY scripts/docker/install-protobuf.bash /
RUN /install-protobuf.bash

# Need Java SDK to build remote parsers
ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64
COPY scripts/docker/install-java.bash /
RUN /install-java.bash
# Make mvn compile quiet
ENV MAVEN_OPTS -Dorg.slf4j.simpleLogger.log.org.apache.maven.cli.transfer.Slf4jMavenTransferListener=warn

# Using the stable version of Hadoop
ENV HADOOP_VERSION 3.2.1
ENV PATH /opt/hadoop-${HADOOP_VERSION}/bin:/usr/local/go/bin:/go/bin:$PATH
COPY scripts/docker/install-hadoop.bash /
RUN /install-hadoop.bash

# Python 3, TensorFlow 2.0.0, etc
COPY scripts/docker/install-python.bash /
RUN /install-python.bash

# Go, goyacc, protoc-gen-go, and other Go tools
ENV GOPATH /root/go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH
COPY scripts/docker/install-go.bash /
RUN /install-go.bash

# ODPS
COPY scripts/docker/install-odps.bash /
RUN /install-odps.bash

# ElasticDL and kubectl
COPY scripts/docker/install-elasticdl.bash /
RUN /install-elasticdl.bash

# The SQLFlow magic command for Jupyter.
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/
COPY scripts/docker/install-jupyter.bash /
COPY scripts/docker/js /js
RUN /install-jupyter.bash
