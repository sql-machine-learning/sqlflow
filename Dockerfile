FROM sqlflow:dev

# Install sample datasets for CI and demo.
COPY doc/datasets/popularize_churn.sql \
     doc/datasets/popularize_iris.sql \
     doc/datasets/popularize_boston.sql \
     doc/datasets/popularize_creditcardfraud.sql \
     doc/datasets/popularize_imdb.sql \
     doc/datasets/create_model_db.sql \
     /docker-entrypoint-initdb.d/
VOLUME /var/lib/mysql

ENV SQLFLOWPATH=$GOPATH/src/sqlflow.org/sqlflow
ENV PYTHONPATH $SQLFLOWPATH/python
ENV SQLFLOW_PARSER_SERVER_PORT 12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH /opt/sqlflow/parser

ARG WITH_SQLFLOW_MODELS="ON"
# Install latest sqlflow_models for testing custom models, see main_test.go:CaseTrainCustomModel
# NOTE: The sqlflow_models works well on the specific Tensorflow version,
#       we can skip installing sqlflow_models if using the older Tensorflow.
RUN if [ "${WITH_SQLFLOW_MODELS:-ON}" = "ON" ]; then \
  git clone https://github.com/sql-machine-learning/models.git && \
  cd models && \
  git checkout c897963f821d515651de79cb4ef1fbf6126ecaa5 && \
  python setup.py bdist_wheel && \
  pip install dist/*.whl && \
  cd .. && \
  rm -rf models; \
fi

 # Expose MySQL server, SQLFlow gRPC server, and Jupyter Notebook server port
EXPOSE 3306
EXPOSE 50051 
EXPOSE 8888

ADD scripts/start.sh /
CMD ["bash", "/start.sh"]
