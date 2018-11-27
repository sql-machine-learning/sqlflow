FROM jupyter/minimal-notebook

USER root

RUN apt-get update && apt-get install -y golang
ENV GOPATH $HOME/go

RUN mkdir -p $GOPATH/src/work
COPY server/server.go $GOPATH/src/work/
COPY client/client.go $GOPATH/src/work/

ENV PKG_CONFIG_PATH /opt/conda/lib/pkgconfig
RUN go get github.com/pebbe/zmq4

ENV LD_LIBRARY_PATH /opt/conda/lib/

COPY entrypoint.sh /
ENTRYPOINT ["/entrypoint.sh"]
