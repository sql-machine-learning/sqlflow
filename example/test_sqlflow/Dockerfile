FROM sqlflow/sqlflow:latest
RUN python -m pip install --upgrade pip
RUN pip install shap
RUN pip install -U sqlflow
# Prepare sample datasets
COPY  ./DNN_model/titanic.sql  /docker-entrypoint-initdb.d/

