#!/bin/bash

# Call the mysql/mysql-server:8.0 default entrypoint to start mysqld.
/entrypoint.sh mysqld

# Create database, table, and popularize the table.
cat /create_table.sql     | mysql -uroot -proot
cat /popularize_table.sql | mysql -uroot -proot
