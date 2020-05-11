FROM ubuntu:18.04

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
