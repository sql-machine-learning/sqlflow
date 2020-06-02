FROM ubuntu:18.04

COPY find_fastest_resources.sh /usr/local/bin/

# The default Ubuntu apt-get source archive.ubuntu.com is usually busy
# and slow.  If you are in the U.S., you might want to use
# http://us.archive.ubuntu.com/ubuntu/, or if you are in China, you
# can try https://mirrors.tuna.tsinghua.edu.cn/ubuntu/
ARG APT_MIRROR="http://us.archive.ubuntu.com/ubuntu/"
ENV APT_MIRROR=$APT_MIRROR
COPY install.sh /
RUN /install.sh

# Java
ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64

# Go
ENV GOPATH /root/go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH

# Set build.sh as the entrypoint and assume that SQLFlow source tree
# is at /work.
COPY build.sh /
CMD ["/build.sh", "/work"]
