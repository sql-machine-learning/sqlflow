# NOTE: The docker build context directory must be the root of the source tree.
# NOTE: To build the release image, SQLFlow must be built into ./build
FROM ubuntu:18.04

# Install and configure the pre-built SQLFlow binaries.
COPY build /build
ENV SQLFLOW_PARSER_SERVER_PORT=12300
ENV SQLFLOW_PARSER_SERVER_LOADING_PATH="/usr/local/sqlflow/java"

# This image includes the SQLFlow gRPC server and external parser servers.  The
# latter is in Java, so we need JRE.
RUN apt-get -qq update \
	&& apt-get -qq install -y openjdk-8-jre-headless \
	&& mv /build/*.jar "$SQLFLOW_PARSER_SERVER_LOADING_PATH" \
	&& mv /build/sqlflowserver /build/sqlflow /build/step /usr/local/bin/

# Expose SQLFlow gRPC server and Jupyter Notebook server port.
EXPOSE 50051

CMD ["sqlflowserver"]
