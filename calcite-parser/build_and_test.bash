#!/bin/bash

set -e # Exit on error.

protoc --java_out=. CalciteParser.proto
protoc --grpc-java_out=. CalciteParser.proto
javac *.java
java CalciteParserServer &
java CalciteParserClient
