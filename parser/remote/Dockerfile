FROM ubuntu:18.04

# Install Java SDK.
RUN apt-get update
RUN apt-get install -y openjdk-8-jdk openjfx

# Install protobuf-compiler with Java support.
RUN apt-get install -y wget unzip
RUN wget -q https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip
RUN unzip -qq protoc-3.7.1-linux-x86_64.zip -d /usr/local
RUN rm protoc-3.7.1-linux-x86_64.zip

# Install gRPC for Java as a protobuf-compiler plugin. c.f. https://stackoverflow.com/a/53982507/724872.
RUN wget -q http://central.maven.org/maven2/io/grpc/protoc-gen-grpc-java/1.19.0/protoc-gen-grpc-java-1.19.0-linux-x86_64.exe
RUN mv protoc-gen-grpc-java-1.19.0-linux-x86_64.exe /usr/local/bin/protoc-gen-grpc-java
RUN chmod +x /usr/local/bin/protoc-gen-grpc-java

# Install all dependencies as JAR files in /deps.
ENV CLASSPATH=$CLASSPATH:/deps/*:.

# Dependencies: Calcite.
RUN wget -q -P /deps http://central.maven.org/maven2/net/hydromatic/aggdesigner-algorithm/6.0/aggdesigner-algorithm-6.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/antlr/antlr-runtime/3.5.2/antlr-runtime-3.5.2.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/calcite/avatica/avatica-core/1.13.0/avatica-core-1.13.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/calcite/avatica/avatica-metrics/1.13.0/avatica-metrics-1.13.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/backport-util-concurrent/backport-util-concurrent/3.1/backport-util-concurrent-3.1.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/calcite/calcite-avatica/1.6.0/calcite-avatica-1.6.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/calcite/calcite-core/1.19.0/calcite-core-1.19.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/calcite/calcite-linq4j/1.19.0/calcite-linq4j-1.19.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/commons-codec/commons-codec/1.10/commons-codec-1.10.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/codehaus/janino/commons-compiler/3.0.11/commons-compiler-3.0.11.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/commons/commons-dbcp2/2.5.0/commons-dbcp2-2.5.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/commons-io/commons-io/2.4/commons-io-2.4.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/commons/commons-lang3/3.8/commons-lang3-3.8.jar
RUN wget -q -P /deps http://central.maven.org/maven2/commons-logging/commons-logging/1.2/commons-logging-1.2.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/commons/commons-pool2/2.6.0/commons-pool2-2.6.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/esri/geometry/esri-geometry-api/2.2.0/esri-geometry-api-2.2.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/google/guava/guava/22.0/guava-22.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/hamcrest/hamcrest-core/1.3/hamcrest-core-1.3.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/httpcomponents/httpclient/4.5.6/httpclient-4.5.6.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/httpcomponents/httpcore/4.4.10/httpcore-4.4.10.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/mnode/ical4j/ical4j/1.0.2/ical4j-1.0.2.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/fasterxml/jackson/core/jackson-annotations/2.9.8/jackson-annotations-2.9.8.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/fasterxml/jackson/core/jackson-core/2.9.8/jackson-core-2.9.8.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/fasterxml/jackson/core/jackson-databind/2.9.8/jackson-databind-2.9.8.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/fasterxml/jackson/dataformat/jackson-dataformat-yaml/2.9.8/jackson-dataformat-yaml-2.9.8.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/codehaus/janino/janino/3.0.11/janino-3.0.11.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/jayway/jsonpath/json-path/2.4.0/json-path-2.4.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/jsoup/jsoup/1.11.3/jsoup-1.11.3.jar
RUN wget -q -P /deps http://central.maven.org/maven2/junit/junit/4.12/junit-4.12.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/yahoo/datasketches/memory/0.9.0/memory-0.9.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/joestelmach/natty/0.13/natty-0.13.jar
RUN wget -q -P /deps http://central.maven.org/maven2/net/sf/opencsv/opencsv/2.3/opencsv-2.3.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/google/protobuf/protobuf-java/3.7.1/protobuf-java-3.7.1.jar
RUN wget -q -P /deps http://central.maven.org/maven2/com/yahoo/datasketches/sketches-core/0.9.0/sketches-core-0.9.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/slf4j/slf4j-api/1.7.25/slf4j-api-1.7.25.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/slf4j/slf4j-simple/1.7.25/slf4j-simple-1.7.25.jar # https://stackoverflow.com/a/9919375
RUN wget -q -P /deps http://central.maven.org/maven2/org/yaml/snakeyaml/1.23/snakeyaml-1.23.jar
RUN wget -q -P /deps http://central.maven.org/maven2/org/apache/calcite/avatica/avatica/1.13.0/avatica-1.13.0.jar

# Dependencies: gRPC and OpenCensus.
RUN wget -q -P /deps http://central.maven.org/maven2/io/grpc/grpc-core/1.20.0/grpc-core-1.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/io/grpc/grpc-netty-shaded/1.20.0/grpc-netty-shaded-1.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/io/grpc/grpc-protobuf/1.20.0/grpc-protobuf-1.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/io/grpc/grpc-protobuf-lite/1.20.0/grpc-protobuf-lite-1.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/io/grpc/grpc-stub/1.20.0/grpc-stub-1.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/io/grpc/grpc-context/1.20.0/grpc-context-1.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/io/opencensus/opencensus-api/0.20.0/opencensus-api-0.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/io/opencensus/opencensus-contrib-grpc-metrics/0.20.0/opencensus-contrib-grpc-metrics-0.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/io/opencensus/opencensus-contrib-http-util/0.20.0/opencensus-contrib-http-util-0.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/io/opencensus/opencensus-impl-core/0.20.0/opencensus-impl-core-0.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/io/opencensus/opencensus-contrib-monitored-resource-util/0.20.0/opencensus-contrib-monitored-resource-util-0.20.0.jar
RUN wget -q -P /deps http://central.maven.org/maven2/commons-cli/commons-cli/1.4/commons-cli-1.4.jar

