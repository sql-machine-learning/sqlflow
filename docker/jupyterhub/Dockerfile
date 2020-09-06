FROM jupyterhub/jupyterhub:1.2

ARG SQLFLOW_MYSQL_IMAGE="sqlflow/sqlflow:mysql"
ENV SQLFLOW_MYSQL_IMAGE=${SQLFLOW_MYSQL_IMAGE}

ARG SQLFLOW_JUPYTER_IMAGE="sqlflow/sqlflow:jupyter"
ENV SQLFLOW_JUPYTER_IMAGE=${SQLFLOW_JUPYTER_IMAGE}

RUN pip install --upgrade pip && pip install jupyterhub-kubespawner \
    oauthenticator==0.11.0 \
    jupyterhub_idle_culler==1.0

COPY docker/jupyterhub/jupyterhub_config.py /etc/jhub/jupyterhub_config.py
# Change logo
RUN sed -i -e 's@{{base_url}}logo@https://avatars0.githubusercontent.com/u/48231449?s=200\&v=4@g' /usr/local/share/jupyterhub/templates/page.html
# Add welcome words
RUN sed -i -e 's@{% block main %}@& \n\
<div style="width:100%; text-align:center; margin:15vh 0 -15vh 0;"> \n\
  <span style="font-size: 2em; color: #1BA2FF">SQLFlow</span> \n\
  <div style="font-size: 1.2em; color: #454545">Extends SQL to support AI. Extract knowledge from Data.</div> \n\
  <br/> \n\
  <div><a href="https://sql-machine-learning.github.io/sqlflow/">Please visit our project on GitHub!</a></div> \n\
</div>@g' /usr/local/share/jupyterhub/templates/login.html

CMD ["jupyterhub", "--config", "/etc/jhub/jupyterhub_config.py"]