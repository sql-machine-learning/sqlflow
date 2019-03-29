#!/bin/bash


nohup sh /cmd/start_namenode.sh &
nohup sh /cmd/start_datanode.sh &
nohup sh /cmd/start_resourcemanager.sh &
nohup sh /cmd/start_nodemanager.sh &
nohup sh /cmd/start_hiveserver2.sh &

schematool -dbType mysql -initSchema --verbose
