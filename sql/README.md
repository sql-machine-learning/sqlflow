# `sql`

This package implements a SQL parser in Go, which is used to parse the extended SELECT syntax of SQL that integrates TensorFlow Estimators.

## Related Work

### Lexer and Parser Generator

In 2001, when I was in graduate school, I defined an extended SQL syntax for querying distributed relational databases, as part of the course project of Distributed RDBMS by Prof. Li-zhu Zhou.  I wrote the parser using [`bison` (a modern version of `yacc`) and `flex` (a modern version of `lex`)](http://dinosaur.compilertools.net/).  `yacc` and `lex` generate C code; `bison` and `flex` generate C++ code. However, this time, I'd use Go.

I surveyed [`goyacc`](https://godoc.org/golang.org/x/tools/cmd/goyacc), a standard Go tool.  The usage is very similar to that of `yacc` and `bison`.  However, the Go toolchain doesn't provide a tool like `lex`/`flex`.

Google revealed [`golex`](https://github.com/cznic/golex), which is out of maintenance.

The [Medium post](https://medium.com/@mhamrah/lexing-with-ragel-and-parsing-with-yacc-using-go-81e50475f88f) recommends [Ragel](http://www.colm.net/open-source/ragel/), which is a C++ program and could generate Go lexer; however, it lacks documents.

### Handwritten Lexer and Parser

Some documents, including [this one](https://hackthology.com/writing-a-lexer-in-go-with-lexmachine.html) recommends handwriting lexers.  However, it doesn't explain how to write the parser.

GoAcademy always provides high-=quality tech blog posts.  [This one](https://blog.gopheracademy.com/advent-2014/parsers-lexers/) is from the author of [InfluxDB](https://github.com/influxdata/influxdb).  However, I stopped at where it explains wrapping a SQL statement as a string by an `io.Reader`, because it is obvious that we should keep the string as a string so that that token strings could refer to the same memory storage of the SQL statement.

Following a link in the above GoAcademy post, I found Rob Pike's excellent talk on how to write a lexer in Go in 2011.  Many works after that change Rob's implementation somehow but always lead to longer and less comprehensible codebases.  

Therefore, I wrote this implementation, which is very loyal to Rob Pike's implementation. More inspiringly, I wrote the parser followed the way Rob Pike wrote the lexer.