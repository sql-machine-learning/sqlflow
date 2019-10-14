FROM sqlflow/sqlflow:latest

# Prepare sample datasets
COPY titanic_dnn/titanic.sql carprice_xgboost/carprice.sql activepower_clustering/activepower.sql /docker-entrypoint-initdb.d/

COPY titanic_dnn/README.md /didi_tutorial/titanic_dnn.md
COPY carprice_xgboost/README.md /didi_tutorial/carprice_xgboost.md
COPY activepower_clustering/README.md /didi_tutorial/activepower_clustering.md
COPY carprice_xgboost/imgs /didi_workspace/imgs

# Convert markdown files into SQLFlow ipynb tutorials
RUN curl -fsSL "https://raw.githubusercontent.com/sql-machine-learning/sqlflow/develop/scripts/convert_markdown_into_ipynb.sh" -o /convert_markdown_into_ipynb.sh && \
    SRC_FOLDER=/didi_tutorial DEST_FOLDER=/didi_workspace bash /convert_markdown_into_ipynb.sh
ENV SQLFLOW_NOTEBOOK_DIR=/didi_workspace

CMD ["bash", "/start.sh"]

