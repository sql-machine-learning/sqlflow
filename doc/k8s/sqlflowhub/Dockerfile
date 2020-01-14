FROM sqlflow/sqlflow

# Installed npm
RUN apt-get -y install npm

RUN mkdir -p /etc/jhub && \
    bash -c "pip install jupyterhub && \
    pip install notebook"
    
# Added to get rid of the of the Proxy Config Folder not founf Errror resulting in POD crash    
RUN npm install -g configurable-http-proxy

COPY jupyterhub_config.py /etc/jhub/jupyterhub_config.py

CMD ["jupyterhub", "--config", "/etc/jhub/jupyterhub_config.py"]
