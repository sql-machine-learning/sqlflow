FROM ubuntu:18.04

ARG FIND_FASTED_MIRROR=true

COPY docker/dev/find_fastest_resources.sh /usr/local/bin/

# Install Python, Go, Java and other build tools
COPY docker/dev/install.sh /
RUN /install.sh

# Java
ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64

# Go
ENV GOPATH /root/go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH

# Set build.sh as the entrypoint and assume that SQLFlow source tree
# is at /work.
COPY docker/dev/build.sh /
CMD ["/build.sh", "/work"]
