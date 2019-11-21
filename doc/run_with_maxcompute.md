# How to Connect MaxCompute with SQLFlow

This tutorial explains how to connect SQLFlow with [MaxCompute (a.k.a ODPS)](https://www.alibabacloud.com/product/maxcompute).

## Connect Existing MaxCompute Server

To connect an existed MaxCompute server instance, we need to configure a `datasource` string in the format of   
`maxcompute://{accesskey_id}:{accesskey_secret}@{endpoint}?curr_project={curr_project}&scheme={scheme}`

In the above format,
- `accesskey_id:accesskey_secret` are the API keys for you to access aliyun. You may find it at the [user center of aliyun](https://usercenter.console.aliyun.com/#/manage/ak) after login.
- `endpoint`. You may find it through the [workbench of aliyun](https://workbench.data.aliyun.com/console?#/) and the [configure endpoints](https://www.alibabacloud.com/help/doc-detail/34951.htm). In the workbench page, let's find the region in the workspace block, e.g.`China North 2`. Then, in the configure endpoints page, we may find out the public endpoint corresponding to the region, in this case, is `service.cn-beijing.maxcompute.aliyun-inc.com/api`. 
   
   **Note**: please be aware that the whole endpoint is `http://service.cn-beijing.maxcompute.aliyun-inc.com/api`. We just take `service.cn-beijing.maxcompute.aliyun-inc.com/api` as the endpoint and the protocol(http) as `scheme` for the `datasource`.
- `curr_project` specifies the workspace name. Let's find it out in the basic information of [the workspace setting](https://workbench.data.aliyun.com/console#/).
- `scheme` specifies the connection protocol of the endpoint. Both `http` and `https` are supported. If you need to encrypt your requests, use `https`.

Using the `datasource`, you may launch an all-in-one Docker container by running:  
```bash
> docker run --rm -p 8888:8888 sqlflow/sqlflow bash -c \
"sqlflowserver & \
SQLFLOW_DATASOURCE='maxcompute://{accesskey_id}:{accesskey_secret}@{endpoint}?curr_project={curr_project}&scheme={scheme}' \
SQLFLOW_SERVER=localhost:50051 \
jupyter notebook --ip=0.0.0.0 --port=8888 --allow-root --NotebookApp.token=''"
```

Open `localhost:8888` through a web browser, you will find there are many SQLFlow tutorials, e.g. `iris-dnn.ipynb`. Please follow the tutorials and substitute the data for your use.

## Create a MaxCompute Instance for Testing
Aliyun supplies a development version that is suitable as a testing environment. Follow the tutorial [Create Workspace](https://www.alibabacloud.com/help/doc-detail/74293.htm), we could create a MaxCompute instance for testing. The development version has some capacity limitations. If you wanna play the testing on a large dataset, please turn to the standard version.

Then, according to the above section, we could build a `datasource` and launch a container.
