FROM jupyterhub/jupyterhub:1.2

RUN pip install jupyterhub-kubespawner jupyterhub-dummyauthenticator \
    kubernetes==9.0.0

COPY docker/jupyterhub/jupyterhub_config.py /etc/jhub/jupyterhub_config.py

CMD ["jupyterhub", "--config", "/etc/jhub/jupyterhub_config.py"]