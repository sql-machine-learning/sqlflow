#!/bin/bash

set -e # Exit on error.

protoc --java_out=. CalciteParser.proto
protoc --grpc-java_out=. CalciteParser.proto
javac *.java

java CalciteParserServer &

java CalciteParserTest "SELECT pn FROM p WHERE pId IN (SELECT pId FROM orders WHERE Quantity > 100)"
if [[ $? -ne 0 ]]; then
    echo "Unexpected error";
fi

java CalciteParserTest "SELECT pn FROM p WHERE pId IN (SELECT pId FROM orders WHERE Quantity > 100) TRAIN DNNClassifier"
if [[ $? -ne 0 ]]; then
    echo "Unexpected error";
fi

java CalciteParserTest "SELECT pn FROM p WHERE pId IN (SELECT pId FROM orders WHERE Quantity > 100) Predict DNNClassifier"
if [[ $? -ne 0 ]]; then
    echo "Unexpected error";
fi

java CalciteParserTest "SELECTED pn FROM p Predict DNNClassifier" 2> /dev/null
if [[ $? -eq 0 ]]; then
    echo "Expected parsing error, but got none";
fi

java CalciteParserTest "SELECTED pn FROM p" 2> /dev/null
if [[ $? -eq 0 ]]; then
    echo "Expected parsing error, but got none";
fi

