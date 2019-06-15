# hiveql-parser

This package is inspired by https://github.com/qzchenwl/hiveql-parser.

## Build

For how to build this parser, please refer to [`../README.md`](../README.md).

## Run

Run the following command to verify that the HiveQL parser works.

```bash
java -jar hiveql/target/hiveql-0.1-SNAPSHOT.jar <(echo "select count(*) as count, myfield from &0rz") 2>/dev/null
```

We should expect outputs like

```
[1,39]: line 1:39 cannot recognize input near '&' '0rz' '<EOF>' in join source
```
