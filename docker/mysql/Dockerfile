# This Dockerfile contains MySQL server and example datasets to be
# populated into the server.  To COPY .sql dataset files outside of
# this directory into the image, we need to run docker build from the
# root directory of the SQLFlow source tree.
#
#  cd $(git rev-parse --show-toplevel)
#  docker build -t sqlflow:mysql -f docker/mysqlflow/Dockerfile .
FROM ubuntu:18.04

COPY docker/mysql/install-mysql.bash /
RUN /install-mysql.bash

# Install sample datasets for CI and demo.
COPY doc/datasets/popularize_churn.sql \
     doc/datasets/popularize_iris.sql \
     doc/datasets/popularize_boston.sql \
     doc/datasets/popularize_creditcardfraud.sql \
     doc/datasets/popularize_imdb.sql \
     doc/datasets/create_model_db.sql \
     /datasets/

VOLUME /var/lib/mysql

ADD docker/sqlflow/start.bash /
CMD ["/start.bash"]
