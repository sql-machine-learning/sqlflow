FROM sqlflow/sqlflow

RUN mkdir -p /etc/jhub && \
    bash -c "source /miniconda/bin/activate sqlflow-dev && \
    conda install -c conda-forge jupyterhub && \
    conda install notebook"

COPY jupyterhub_config.py /etc/jhub/jupyterhub_config.py

CMD ["jupyterhub", "--config", "/etc/jhub/jupyterhub_config.py"]