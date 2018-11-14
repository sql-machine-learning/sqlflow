#!/bin/bash

cmp --silent \
    <(cat testdata/train_test.sql | go run sqlparser.go) \
    <(cat testdata/train_test.json) \
    || (echo "JSONs are different"; exit -1)
