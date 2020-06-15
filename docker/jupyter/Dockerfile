# This Dockerfile containers Jupyter Notebook server with many
# SQLFlow tutorials and SQLFlow magic command.

FROM jupyter/base-notebook

# using root user to avoid permission deni
USER root
# Choose fastest mirrors for apt-get and pip
COPY docker/dev/find_fastest_resources.sh /usr/local/bin/find_fastest_resources.sh
RUN /bin/bash -c 'source find_fastest_resources.sh \
  && echo "Choose the fastest APT source ..." \
  && choose_fastest_apt_source \
  && echo "Choose the fastest PIP source ..." \
  && choose_fastest_pip_source'

# Install dependencies.
COPY docker/jupyter/js /jupyter/js

RUN apt-get -qq update

COPY docker/jupyter/install-jupyter.bash /jupyter
RUN /jupyter/install-jupyter.bash

# switch back to the default user to avoid accidental container runs as root,
# this env comes from base Dockerfile:  https://github.com/jupyter/docker-stacks/blob/master/base-notebook/Dockerfile#L13
USER $NB_UID

# Install IPythono Notebook tutorials
COPY build/tutorial /workspace

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
