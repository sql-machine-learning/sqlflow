FROM python:3.7

RUN pip install --upgrade pip && pip install tensorflow mysql-connector-python jupyter sqlflow

# Load sqlflow Jupyter magic command automatically. c.f. https://stackoverflow.com/a/32683001.
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/
RUN mkdir -p $IPYTHON_STARTUP
RUN echo 'get_ipython().magic(u"%reload_ext sqlflow.magic")' >> $IPYTHON_STARTUP/00-first.py
RUN echo 'get_ipython().magic(u"%autoreload 2")' >> $IPYTHON_STARTUP/00-first.py

ADD demo /usr/bin/demo
ADD sqlflowserver /usr/bin/sqlflowserver

CMD ["/usr/bin/demo"]
