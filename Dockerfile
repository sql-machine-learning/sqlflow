FROM ubuntu:16.04

RUN apt-get update
RUN apt-get install -y curl bzip2

# Miniconda - Python 3.6, 64-bit, x86, latest
ARG CONDA_OS=Linux
RUN curl -sL https://repo.continuum.io/miniconda/Miniconda3-latest-Linux-x86_64.sh -o mconda-install.sh && \
    bash -x mconda-install.sh -b -p miniconda && \
    rm mconda-install.sh
ENV PATH="/miniconda/bin:$PATH"

ARG CONDA_ADD_PACKAGES=""
RUN conda create -y -q -n sqlflow-dev python=3.6 ${CONDA_ADD_PACKAGES}

RUN echo ". /miniconda/etc/profile.d/conda.sh" >> ~/.bashrc && \
    echo "source activate sqlflow-dev" >> ~/.bashrc

ARG PIP_ADD_PACKAGES=""
RUN /bin/bash -c "source activate sqlflow-dev && python -m pip install \
    tensorflow==2.0.0-alpha0 \
    mysql-connector-python \
    pyhive \
    jupyter \
    sqlflow \
    ${PIP_ADD_PACKAGES} \
    "
# Fix jupyter server "connecting to kernel" problem
# https://github.com/jupyter/notebook/issues/2664#issuecomment-468954423
RUN /bin/bash -c "source activate sqlflow-dev && python -m pip install tornado==4.5.3"

# Load sqlflow Jupyter magic command automatically. c.f. https://stackoverflow.com/a/32683001.
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/
RUN mkdir -p $IPYTHON_STARTUP
RUN echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")' >> $IPYTHON_STARTUP/00-first.py
RUN echo 'get_ipython().magic(u"%autoreload 2")' >> $IPYTHON_STARTUP/00-first.py

ADD demo /usr/bin/demo
ADD sqlflowserver /usr/bin/sqlflowserver

CMD ["/usr/bin/demo"]
