FROM ubuntu:18.04

# Install Java SDK.
RUN apt-get update && apt-get install -y openjdk-8-jdk openjfx curl git

# Download antlr
RUN cd /usr/local/lib && curl -s -O https://www.antlr.org/download/antlr-4.7.1-complete.jar
ENV CLASSPATH=$CLASSPATH:/usr/local/lib/antlr-4.7.1-complete.jar:.

# Download Go
RUN curl -s https://dl.google.com/go/go1.12.5.linux-amd64.tar.gz | tar -C /usr/local -xzf -
ENV GOPATH=/go
ENV PATH=$PATH:/usr/local/go/bin:$GOPATH/bin

# Install Antlr's Go target
RUN go get github.com/antlr/antlr4/runtime/Go/antlr
RUN cd /go/src/github.com/antlr/antlr4/runtime/Go/antlr && git checkout tags/4.6 && go install 

RUN echo "#!/bin/bash" > /build.sh
RUN echo "java org.antlr.v4.Tool -Dlanguage=Go -package hivesql /work/Hplsql.g4" >> /build.sh
RUN chmod +x /build.sh

VOLUME "/work"
CMD ["/build.sh"]
