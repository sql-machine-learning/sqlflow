FROM sqlflow/sqlflow:latest

# Prepare sample datasets
COPY titanic_dnn/titanic.sql \
        carprice_xgboost/carprice.sql \
        activepower_clustering/activepower.sql \
        /docker-entrypoint-initdb.d/

COPY titanic_dnn/README.md /didi_tutorial/titanic_dnn.md
COPY carprice_xgboost/README.md /didi_tutorial/carprice_xgboost.md
COPY activepower_clustering/README.md /didi_tutorial/activepower_clustering.md
COPY carprice_xgboost/imgs /didi_workspace/imgs

ENV SQLFLOW_NOTEBOOK_DIR=/didi_workspace
RUN echo "Convert tutorials from Markdown to IPython notebooks ..."; \
        mkdir -p $SQLFLOW_NOTEBOOK_DIR; \
for file in /didi_tutorial/*.md; do \
        base=$(basename -- "$file"); \
        output=$SQLFLOW_NOTEBOOK_DIR/${base%.*}."ipynb"; \
        cat $file | markdown-to-ipynb --code-block-type=sql > $output; \
done

CMD ["/start.sh"]
