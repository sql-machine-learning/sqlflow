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

# Install Couler, Fluid, sqlflow_submitter and model zoo.
COPY build/*.whl /usr/local/sqlflow/python/
RUN pip install /usr/local/sqlflow/python/*.whl

# Install the pre-built binaries
COPY build/sqlflowserver /usr/local/bin
COPY build/sqlflow /usr/local/bin
COPY build/step /usr/local/bin

# Install the Java gRPC parser servers.
COPY build/*.jar /usr/local/sqlflow/java/
ENV SQLFLOW_PARSER_SERVER_PORT 12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH /usr/local/sqlflow/java

# Install the tutorials
COPY build/tutorial /workspace

 # Expose MySQL server, SQLFlow gRPC server, and Jupyter Notebook server port
EXPOSE 3306
EXPOSE 50051
EXPOSE 8888

ADD scripts/start.sh /
CMD ["bash", "/start.sh"]
