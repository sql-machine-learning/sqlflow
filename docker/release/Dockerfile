# NOTE: The docker build context directory must be the root of the source tree.
# NOTE: To build the release image, SQLFlow must be built into ./build
FROM ubuntu:18.04

# Install and configure the pre-built SQLFlow binaries.
COPY build /build
ENV SQLFLOW_PARSER_SERVER_PORT=12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH="/usr/local/sqlflow/java"
COPY docker/release/install.sh /install.sh
RUN /install.sh
WORKDIR /workspace

# Expose SQLFlow gRPC server and Jupyter Notebook server port.
EXPOSE 50051

CMD ["sqlflowserver"]
