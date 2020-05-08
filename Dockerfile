FROM ubuntu:18.04

# The default source archive.ubuntu.com is busy and slow. We use the following source makes docker build running faster.
RUN echo '\n\
deb http://us.archive.ubuntu.com/ubuntu/ bionic main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ bionic-security main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ bionic-updates main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ bionic-proposed main restricted universe multiverse \n\
deb http://us.archive.ubuntu.com/ubuntu/ bionic-backports main restricted universe multiverse \n\
' > /etc/apt/sources.list

RUN apt-get -qq update

# Install wget, curl, unzip, bzip2, git
COPY scripts/docker/install-download-tools.bash /
RUN /install-download-tools.bash

# Install shellcheck
COPY scripts/docker/install-shell-tools.bash /
RUN /install-shell-tools.bash

# MySQL server and client
COPY scripts/docker/install-mysql.bash /
RUN /install-mysql.bash
COPY doc/datasets/popularize_churn.sql \
     doc/datasets/popularize_iris.sql \
     doc/datasets/popularize_boston.sql \
     doc/datasets/popularize_creditcardfraud.sql \
     doc/datasets/popularize_imdb.sql \
     doc/datasets/create_model_db.sql \
     /docker-entrypoint-initdb.d/
VOLUME /var/lib/mysql

# Install protobuf and protoc
COPY scripts/docker/install-protobuf.bash /
RUN /install-protobuf.bash

# Need Java SDK to build remote parsers
ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64
COPY scripts/docker/install-java.bash /
RUN /install-java.bash
# Make mvn compile quiet
ENV MAVEN_OPTS -Dorg.slf4j.simpleLogger.log.org.apache.maven.cli.transfer.Slf4jMavenTransferListener=warn

# Using the stable version of Hadoop
ENV HADOOP_VERSION 3.2.1
ENV PATH /opt/hadoop-${HADOOP_VERSION}/bin:/usr/local/go/bin:/go/bin:$PATH
COPY scripts/docker/install-hadoop.bash /
RUN /install-hadoop.bash

# Python 3, TensorFlow 2.0.0, etc
COPY scripts/docker/install-python.bash /
RUN /install-python.bash

# Go, goyacc, protoc-gen-go, and other Go tools
ENV GOPATH /root/go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH
COPY scripts/docker/install-go.bash /
RUN /install-go.bash

# ODPS
COPY scripts/docker/install-odps.bash /
RUN /install-odps.bash

# ElasticDL and kubectl
COPY scripts/docker/install-elasticdl.bash /
RUN /install-elasticdl.bash

# The SQLFlow magic command for Jupyter.
ENV IPYTHON_STARTUP /root/.ipython/profile_default/startup/
COPY scripts/docker/install-jupyter.bash /
COPY scripts/docker/js /js
RUN /install-jupyter.bash

# -----------------------------------------------------------------------------------
# Above Steps Should be Cached for Each CI Build if Dockerfile is not Changed.
# -----------------------------------------------------------------------------------

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
