FROM sqlflow/sqlflow:latest
# Prepare sample datasets
COPY  titanic/titanic.sql   carprice/carprice.sql   /docker-entrypoint-initdb.d/
ADD carprice/tutorial_xgboost_carprice.ipynb /workspace/
ADD titanic/tutorial_dnn_titanic.ipynb /workspace/
CMD ["bash", "/start.sh"]

