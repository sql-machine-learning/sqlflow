FROM tensorflow/tensorflow:1.12.0

RUN pip install --upgrade pip
RUN pip install mysql-connector-python
