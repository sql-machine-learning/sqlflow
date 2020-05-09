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

RUN apt-get -qq update

# Install wget, curl, unzip, bzip2, git
COPY install-download-tools.bash /
RUN /install-download-tools.bash

# Install shellcheck
COPY install-shell-tools.bash /
RUN /install-shell-tools.bash

# MySQL server and client
COPY install-mysql.bash /
RUN /install-mysql.bash

# Install protobuf and protoc
COPY install-protobuf.bash /
RUN /install-protobuf.bash

# Need Java SDK to build remote parsers
ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64
COPY install-java.bash /
RUN /install-java.bash

# Using the stable version of Hadoop
ENV HADOOP_VERSION 3.2.1
ENV PATH /opt/hadoop-${HADOOP_VERSION}/bin:/usr/local/go/bin:/go/bin:$PATH
COPY install-hadoop.bash /
RUN /install-hadoop.bash

# Python 3, TensorFlow 2.0.0, etc
COPY install-python.bash /
RUN /install-python.bash

# Go, goyacc, protoc-gen-go, and other Go tools
ENV GOPATH /root/go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH
COPY install-go.bash /
RUN /install-go.bash

# ODPS
COPY install-odps.bash /
RUN /install-odps.bash

# ElasticDL and kubectl
COPY install-elasticdl.bash /
RUN /install-elasticdl.bash

# The SQLFlow magic command for Jupyter.
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/
COPY install-jupyter.bash /
COPY js /js
RUN /install-jupyter.bash

# Set build.sh as the entrypoint and assume that SQLFlow source tree
# is at /work.
COPY build.sh /
CMD ["/build.sh", "/work"]
