FROM jupyterhub/jupyterhub:1.2

ARG SQLFLOW_MYSQL_IMAGE="sqlflow/sqlflow:mysql"
ENV SQLFLOW_MYSQL_IMAGE=${SQLFLOW_MYSQL_IMAGE}

ARG SQLFLOW_JUPYTER_IMAGE="sqlflow/sqlflow:jupyter"
ENV SQLFLOW_JUPYTER_IMAGE=${SQLFLOW_JUPYTER_IMAGE}

RUN pip install --upgrade pip && pip install jupyterhub-kubespawner \
    oauthenticator==0.11.0 \
    jupyterhub-dummyauthenticator==0.3.1 \
    jupyterhub_idle_culler==1.0

COPY docker/jupyterhub/jupyterhub_config.py /etc/jhub/jupyterhub_config.py
COPY docker/jupyterhub/provision.sh /provision.sh
RUN bash /provision.sh

CMD ["jupyterhub", "--config", "/etc/jhub/jupyterhub_config.py"]
