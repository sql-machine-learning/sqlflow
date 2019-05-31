# HiveSQL Parser

This package builds a Go parser from the grammar rule files `Hplsql.g4`, which contains grammar rules of Hive SQL.

## The Grammar File

The grammar file `Hplsql.g4` is based on a [copy](https://github.com/wangkuiyi/hive/blob/release-2.0.1/hplsql/src/main/antlr4/org/apache/hive/hplsql/Hplsql.g4) from Apache Hive's official Github repo with Git tag `release-2.0.1`.  Then we slightly editted the file to rename grammar rule `string` into `char_string`, as `string` conflicts with the name of Go data type `string`. This edit affacts only four lines, as shown in this [pull request](https://github.com/apache/hive/pull/654).

## The Parser

Apache Hive use ANTLR to translate the grammar file into a paser in Java.  Here we use ANTLR and its Go language target to generate a parser in Go.

If you don't want to install related tools on your computer, you might install them into a Docker image using the `Dockerfile`:

```bash
docker build -t hivesql:dev .
```

Then, we can run a container to generate the parser in Go

```bash
docker run --rm -it -v $PWD:/work -w /work hivesql.dev
```

