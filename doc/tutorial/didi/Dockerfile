FROM sqlflow/sqlflow:latest
# Prepare sample datasets

COPY  titanic_dnn/titanic.sql   carprice_xgboost/carprice.sql   /docker-entrypoint-initdb.d/

COPY ./convert_markdown_into_ipynb.sh /convert_markdown_into_ipynb.sh

COPY titanic_dnn/README.md /didi_tutorial/titanic_dnn.md
COPY carprice_xgboost/README.md /didi_tutorial/carprice_xgboost.md

RUN bash /convert_markdown_into_ipynb.sh

CMD ["bash", "/start.sh"]

