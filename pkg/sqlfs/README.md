# `sqlfs`

The package `sqlfs` provides an `io.ReadCloser` and an `io.WriteCloser` that treats a SQL database a filesystem, where each table in the database is like a file.  The schema of the table is very simple -- it has only one column of BLOB type. All the rows consist the storage.

`sqlfs` provides the following features.

## Create a file

To create a table named "hello" in a database "mydb" for writing, we can call `Create`.

```go
f, e := sqlfs.Create(db, "mydb.hello")
f.Write([]byte("hello world!\n"))
f.Close()
```

where `db` comes from a call to `database/sql.Open`.

## Append to a file

```go
f, e := sqlfs.Append(db, "mydb.hello")
f.Write([]byte("hello world!\n")
f.Close()
```

## Read from a file

```go
f, e := sqlfs.Open(db, "mydb.hello")
buf := make([]byte, 1024)
f.Read(buf)
f.Close()
```

## Remove a file

```go
DropTable(db, "mydb.hello")
```

## Check if a file exists

```go
HasTable(db, "mydb.hello")
```

## Other I/O operations

Feel free to use standard packages `io`, `ioutil`, etc with `sqlfs`.  For example, we can call `io.Copy` to copy everything from the standard input to a table.

```go
f, e := sqlfs.Create(db, "mydb.hello")
io.Copy(f, os.Stdin)
f.Close()
```
