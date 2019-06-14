# Note: I used the official maven Docker image to build this package.
# However, Maven complains it cannot find tools.jar. I verified that
# I'd have to yum install java-8-openjdk in that Docker image to make
# Maven works.  However, I'd follow this alternative to create a
# Docker image from Ubuntu instead of Oracle Linux.
FROM ubuntu:18.04

RUN apt-get update && apt-get install -y openjdk-8-jdk maven openjfx
ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64

# Install protobuf-compiler with Java support.
RUN apt-get install -y wget unzip
RUN wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip
RUN unzip -qq protoc-3.7.1-linux-x86_64.zip -d /usr/local
RUN rm protoc-3.7.1-linux-x86_64.zip

# Install gRPC for Java as a protobuf-compiler plugin. c.f. https://stackoverflow.com/a/53982507/724872.
RUN wget -q http://central.maven.org/maven2/io/grpc/protoc-gen-grpc-java/1.21.0/protoc-gen-grpc-java-1.21.0-linux-x86_64.exe
RUN mv protoc-gen-grpc-java-1.21.0-linux-x86_64.exe /usr/local/bin/protoc-gen-grpc-java
RUN chmod +x /usr/local/bin/protoc-gen-grpc-java
