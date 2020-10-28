# This Dockerfile containers Jupyter Notebook server with many
# SQLFlow tutorials and SQLFlow magic command.

FROM alpine:3.12

ARG FIND_FASTED_MIRROR=true

COPY docker/dev/find_fastest_resources.sh /usr/local/bin/
RUN /bin/sh -c 'if [ "$FIND_FASTED_MIRROR" == "true" ]; then source find_fastest_resources.sh && \
    choose_fastest_alpine_source && \
    choose_fastest_pip_source; fi'

RUN apk add --no-cache python3 python3-dev py3-pip py3-pyzmq py3-grpcio

# Install IPythono Notebook tutorials
COPY /docker/jupyter/js /jupyter/js/
COPY build/tutorial /workspace
# Remove non-interactive tutorials
RUN rm /workspace/energe_lstmbasedtimeseries.ipynb \
    /workspace/cora-gcn.ipynb \
    /workspace/imdb-stackedbilstm.ipynb
COPY docker/jupyter/install-jupyter.sh /jupyter/install-jupyter.sh
RUN /bin/sh /jupyter/install-jupyter.sh

# Cleanup
RUN apk del --purge python3-dev

# The following SQLFlow gRPC server endpoint implies the server runs in a container,
# and if container has the option --net=container:sqlflow_server_container, SQLFlow magic
# command can access the SQLFlow gRPC server running in another container as it runs
# in the same container.
ARG SQLFLOW_SERVER="localhost:50051"
ENV SQLFLOW_SERVER=${SQLFLOW_SERVER}

# The following data source URL implies that the MySQL server runs in
# a container, the data source will be retrieved by SQLFlow magic command
# and be sent to SQLFLow server in each request. The SQLFlow server compiles this
# value into each step container, so these step container knews data source.
ARG SQLFLOW_DATASOURCE="mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0"
ENV SQLFLOW_DATASOURCE=${SQLFLOW_DATASOURCE}

WORKDIR /workspace
EXPOSE 8888

CMD ["jupyter", "notebook", "--ip=0.0.0.0", "--port=8888", "--allow-root", "--NotebookApp.token=''"] 