FROM jupyterhub/jupyterhub:1.2

ARG SQLFLOW_MYSQL_IMAGE="sqlflow/sqlflow:mysql"
ENV SQLFLOW_MYSQL_IMAGE=${SQLFLOW_MYSQL_IMAGE}

ARG SQLFLOW_JUPYTER_IMAGE="sqlflow/sqlflow:jupyter"
ENV SQLFLOW_JUPYTER_IMAGE=${SQLFLOW_JUPYTER_IMAGE}

RUN pip install jupyterhub-kubespawner jupyterhub-dummyauthenticator \
    kubernetes==9.0.0

COPY docker/jupyterhub/jupyterhub_config.py /etc/jhub/jupyterhub_config.py

CMD ["jupyterhub", "--config", "/etc/jhub/jupyterhub_config.py"]