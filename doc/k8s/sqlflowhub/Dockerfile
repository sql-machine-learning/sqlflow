FROM sqlflow/sqlflow

RUN mkdir -p /etc/jhub && \
    bash -c "pip install jupyterhub && \
    pip install notebook"

COPY jupyterhub_config.py /etc/jhub/jupyterhub_config.py

CMD ["jupyterhub", "--config", "/etc/jhub/jupyterhub_config.py"]
