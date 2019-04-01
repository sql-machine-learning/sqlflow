# GoHive:  A Hive Driver for Go

Go programmers usually call the standard package `database/sql` to access databases. `database/sql` relies on [database drivers](https://golang.org/src/database/sql/doc.txt) to work with database management systems.  A growing list of drivers is at https://github.com/golang/go/wiki/SQLDrivers, where we cannot find one for Apache Hive at the moment we wrote this package.

## Walkthrough the Code

- `driver.go`

  As required by `database/sql`, `driver.go` defines type `gohive.Driver` that implements the `database/sql/driver.Driver` interface, which has a method `Open`.  The `init()` method [registers](https://golang.org/pkg/database/sql/#Register) the type `gohive.Driver`.

- `connection.go`

  The `Open` method of `database/sql/driver.Driver` is supposed to return a connection, which is a type that implements the [`database/sql/driver.Conn`](https://golang.org/pkg/database/sql/driver/#Conn) interface.  `connection.go` defines the type `gohive.Driver` that implements `database/sql/driver.Conn`.

  `gohive.Driver` also implements [`database/sql/driver.QueryerContext`](https://golang.org/pkg/database/sql/driver/#QueryerContext) by defining method `QueryContext` to run queries and to return rows.

- `rows.go`

  `rows.go` defines type `gohive.Rows` that implements the interface [`database/sql/driver.Rows`](https://golang.org/pkg/database/sql/driver/#Rows).


## Access Hive through Thrift

For contributors who are curious how this driver talks to Hive via its Thrift interface, [here](https://github.com/apache/hive/blob/master/service-rpc/if/TCLIService.thrift) is Hive's Thrift service definition.

## Running Hive in Containers

For the convenience of the development of this package, we provide [Dockerfiles](dockerfile) that install and run Hive in Docker containers.
