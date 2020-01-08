FROM sqlflow/sqlflow

RUN echo "root:root" | chpasswd
RUN sudo -s

RUN mkdir /home/sqlflow
RUN chown root:root /home/sqlflow
RUN chmod -R 755 /home/sqlflow
RUN chmod -R 755 /root

RUN mkdir -p /etc/jhub && \
    bash -c "pip install jupyterhub && \
    pip install notebook"

COPY jupyterhub_config.py /etc/jhub/jupyterhub_config.py

CMD ["jupyterhub", "--config", "/etc/jhub/jupyterhub_config.py"]
