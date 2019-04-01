# Standalone Mode
## 1. create all-in-one docker image using dockerfile
```
cd gohive
docker build -t gohive dockerfile/all_in_one
```
## 2. run and login the docker container
```
sh dockerfile/all_in_one/dev.sh
```
## 3. download gohive source code
```
env GIT_TERMINAL_PROMPT=1 go get github.com/wangkuiyi/sqlflow
cd /go/src/github.com/wangkuiyi/sqlflow/gohive
/go/bin/dep ensure
```
## 4. compile and run the gohive
```
go build
go test
```
# Distribution Mode
**notes:** in distribution mode, the hive docker container and go client docker container should be on the same physical machine because the hive go thrift client in go client docker default use 127.0.0.1:10000 to connect hiveserver2 and you could change the hiveserver2 address in driver_test.go.
## 1. create hive docker image using dockerfile
```
cd gohive
docker build -t hive_ubuntu dockerfile/hive
```
## 2. run and login the hive docker container
```
sh dockerfile/hive/dev.sh
```
## 3. check if hiveserver2 start successfully
execute the commands below in the docker.

```
# check hiveserver2 port (if not ready, just wait for several minutes)
netstat -apn | grep 10000

# check if the test db is ready.
hive -e "select * from churn.train"
```

## 4. create go_client docker image using dockerfile
```
cd gohive
docker build -t go_client dockerfile/go_client
```
## 5. run and login the go_client docker container
```
sh dockerfile/go_client/dev.sh
```
## 6. check if the golang installed successfully
```
go version
```
## 7. download gohive source code
```
env GIT_TERMINAL_PROMPT=1 go get github.com/wangkuiyi/sqlflow
cd /go/src/github.com/wangkuiyi/sqlflow/gohive
/go/bin/dep ensure
```
## 8. compile and run the gohive
```
go build
go test
```