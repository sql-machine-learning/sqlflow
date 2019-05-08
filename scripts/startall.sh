#!/bin/bash

# Start mysqld
docker-entrypoint.sh mysqld &
sleep 1
until mysql -u root -proot  -e ";" ; do
    sleep 1
    read -p "Can't connect, retrying..."
done
# FIXME(typhoonzero): should let docker-entrypoint.sh do this work
for f in /docker-entrypoint-initdb.d/*; do
    cat $f | mysql -uroot -proot
done
# Start sqlflowserver
sqlflowserver --datasource='mysql://root:root@tcp(127.0.0.1:3306)/?maxAllowedPacket=0' &
# Start jupyter notebook
SQLFLOW_SERVER=localhost:50051 jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root
