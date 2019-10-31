Overview:
This is a doc on integration with katib XGboost.

User Interface:

Implementation:

Steps:

1.Based on input SQL statement, the codegen generates a katib-xgboost.py file and is submitted to SQLflow server.
2.SQLflow server executes katib-xgboost.py:
	1.to generate train_xgboost.py file. All input parameters are filled in train_xgboost.py.
	2.to generate a Dockerfile and build a docker image based on this Dockerfile. The train_xgboost.py and required data file will be copied into this image at the same time.
	3.to push this docker image into docker.io repository.
	4.to generate a katib-xgboost.yaml and fill it with:
		1.source of docker image generated above.
		2.commands to execute train_xgboost in the container.
	5.to submit and execute katib-xgboost.yaml on kubernetes.
3.Katib creates an experiments to run XGboost training job.

