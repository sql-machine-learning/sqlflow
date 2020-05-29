# To build the release image, SQLFlow must be built into ./build
FROM ubuntu:18.04

# Install pre-built SQLFlow components.
COPY build /build
ENV SQLFLOW_PARSER_SERVER_PORT=12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH="/usr/local/sqlflow/java"
RUN pip install --quiet /build/*.whl \
        && mv /build/sqlflowserver /build/sqlflow /build/step /usr/local/bin/ \
        && mkdir -p $SQLFLOW_PARSER_SERVER_LOADING_PATH \
        && mv /build/*.jar $SQLFLOW_PARSER_SERVER_LOADING_PATH \
        && mv /build/tutorial /workspace
WORKDIR /workspace

# Expose SQLFlow gRPC server and Jupyter Notebook server port.
EXPOSE 50051

CMD ["sqlflowserver"]
