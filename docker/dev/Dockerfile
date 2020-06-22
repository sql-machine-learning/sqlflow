FROM ubuntu:18.04

COPY docker/dev/find_fastest_resources.sh /usr/local/bin/

# Install Python, Go, Java and other build tools
COPY docker/dev/install.sh /
RUN /install.sh

# Java
ENV JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64

# Go
ENV GOPATH /root/go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH

# NOTE(sneaxiy): CI would run "pre-commit run", and
# "pre-commit run" would call "docgen.go" to generate
# "model_parameter.md". "docgen.go" would call "python symbol_extractor.py"
# to extract the symbols from TensorFlow and XGBoost pre-defined
# models. So we must install the Python source code to run
# "symbol_extractor.py" here, i.e., make "pre-commit run" run successfully on CI.
COPY python /usr/local/sqlflow/python
ENV PYTHONPATH=/usr/local/sqlflow/python:$PYTHONPATH

# Set build.sh as the entrypoint and assume that SQLFlow source tree
# is at /work.
COPY docker/dev/build.sh /
CMD ["/build.sh", "/work"]
