# HiveServer 2 Client in Go

For the convenience to access Hive from clients in various languages, the Hive developers created Hive Server, which is a Thrift service.  The currently well-used version is known as Hive Server 2.

To write a Hive Server 2 client in Go, we need to use the `thrift` command to compile the Thrift service definition file [`TCLIService.thrift`](https://github.com/apache/hive/blob/master/service-rpc/if/TCLIService.thrift) from Hive Server 2 codebase, into Go source code.

According to their [blog post](https://cwiki.apache.org/confluence/display/Hive/HowToContribute), the Hive developers for some reasons locks the Thrift version to 0.9.3, which is pretty old that you might not want to install it on your computer.  Thanks to the Thrift team, who releases Thrift in Docker images and we can use the 0.9.3 version of Docker image for the compilation:

```bash
docker run --rm -it -v $PWD:/work -w /work thrift:0.9.3 thrift -r --gen go if/TCLIService.thrift
```

The above command generates Go source code in the subdirectory `./gen-go/tcliservice`.

It doesn't look very probable for the Hive team to upgrade the Thrift version or the `TCLIService.thrift` file, so we don't expect that you might need to run the above command, and we include the generated Go source files in this Git repo.
