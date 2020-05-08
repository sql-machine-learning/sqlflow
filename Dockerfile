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

# Build SQLFlow, copy sqlflow_submitter, install Java parser (129 MB), convert tutorial markdown to ipython notebook
ENV SQLFLOWPATH $GOPATH/src/sqlflow.org/sqlflow
ENV PYTHONPATH $SQLFLOWPATH/python
ENV SQLFLOW_PARSER_SERVER_PORT 12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH /opt/sqlflow/parser
COPY . $SQLFLOWPATH
RUN cd $SQLFLOWPATH && \
go generate ./... && \
go install -v ./... && \
mv $GOPATH/bin/sqlflowserver /usr/local/bin && \
mv $GOPATH/bin/sqlflow /usr/local/bin && \
mv $GOPATH/bin/step /usr/local/bin && \
(cd python/couler && python setup.py -q install) && \
(git clone https://github.com/sql-machine-learning/fluid.git && cd fluid && git checkout ceda474 && python setup.py bdist_wheel && pip install dist/*.whl) && \
mkdir -p $SQLFLOW_PARSER_SERVER_LOADING_PATH && \
(cd java/parse-interface && mvn clean install -B) && \
(cd java/parser-hive && mvn -B -q clean compile assembly:single && mv target/*.jar $SQLFLOW_PARSER_SERVER_LOADING_PATH) && \
(cd java/parser-calcite && mvn -B -q clean compile assembly:single && mv target/*.jar $SQLFLOW_PARSER_SERVER_LOADING_PATH) && \
(cd java/parser && \
protoc --java_out=src/main/java --grpc-java_out=src/main/java/ --proto_path=src/main/proto/ src/main/proto/parser.proto && \
mvn -B -q clean compile assembly:single && \
cp target/*.jar $SQLFLOW_PARSER_SERVER_LOADING_PATH) && \
cd / && \
bash $SQLFLOWPATH/scripts/convert_markdown_into_ipynb.sh

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
