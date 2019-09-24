FROM sqlflow/sqlflow:latest

# Prepare sample datasets
COPY  titanic_dnn/titanic.sql   carprice_xgboost/carprice.sql   /docker-entrypoint-initdb.d/
COPY titanic_dnn/README.md /didi_tutorial/titanic_dnn.md
COPY carprice_xgboost/README.md /didi_tutorial/carprice_xgboost.md

# Convert markdown files into SQLFlow ipynb tutorials
RUN curl -fsSL "https://raw.githubusercontent.com/sql-machine-learning/sqlflow/develop/scripts/convert_markdown_into_ipynb.sh" -o /convert_markdown_into_ipynb.sh && \
    SRC_FOLDER=/didi_tutorial bash /convert_markdown_into_ipynb.sh

CMD ["bash", "/start.sh"]

