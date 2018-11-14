#!/bin/bash

cmp --silent \
    <(cat train_test.sql | go run sqlparser.go) \
    <(cat train_test.json) \
    || (echo "JSONs are different"; exit -1)
