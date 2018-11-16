# ODBCDataset

ODBCDataset is a TensorFlow dataset operator that launches a SQL SELECT statement on a SQL engine and reads the result through ODBC.  This operator enables TensorFlow graphs to read data from SQL engines.

## Related Work

https://bobobobo.wordpress.com/2009/07/11/working-with-odbc-from-c/ shows how a C++ program could access MySQL via ODBC, OLE, and DAO.

https://docs.microsoft.com/en-us/azure/sql-database/sql-database-develop-cplusplus-simple explains how to write a C++ program on Linux to access Microsoft SQL Server and Azure SQL via ODBC.  To install the Microsoft ODBC driver `apt-get install msodbcsql`.

https://dev.mysql.com/doc/connector-odbc/en/connector-odbc-installation-binary-unix.html explains how to install the MySQL ODBC driver, and 



## How to Build

Following the suggestions in TensorFlow's [official document](https://www.tensorflow.org/extend/new_data_formats), we build ODBCDataset into a shared library that can be imported by a TensorFlow Python program.

The following steps build the operators for TensorFlow 1.11.0:

1. In the `/tensorflow/odbc_dataset` directory, launch a TensorFlow development Docker container:

   ```bash
   docker run --rm -it -v $PWD:/work -w /work tensorflow/tensorflow:1.11.0-devel bash
   ```
   
1. In the container, run the following commands copied from the [operator building document](https://www.tensorflow.org/extend/adding_an_op) to setup the environment variables:

   ```bash
   TF_CFLAGS=( $(python -c 'import tensorflow as tf; print(" ".join(tf.sysconfig.get_compile_flags()))') )
   TF_LFLAGS=( $(python -c 'import tensorflow as tf; print(" ".join(tf.sysconfig.get_link_flags()))') )
   ```
   
1. Everytime the source file is modified, rebuild the shared library:

   ```bash
   g++ -std=c++11 -shared odbc_dataset.cc -o odbc_dataset.so -fPIC ${TF_CFLAGS[@]} ${TF_LFLAGS[@]} -O2
   ```
