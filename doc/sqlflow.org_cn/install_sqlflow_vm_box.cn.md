# 安装 SQLFlow VM Box

## 安装
SQLFlow 的运行依赖大量的组件，我们已经将这些依赖打包到一个虚拟机镜像中，用户只需要安装该镜像
即可快速体验 SQLFlow 功能。请参考以下步骤进行安装：

1. 安装 [VirtualBox](https://www.virtualbox.org/wiki/Downloads) (推荐v6.1.6版本)
1. 下载预制的 SQLFlow 镜像，这里你有两个选择：
    * 精简版（600M），在这个版本中，我们预置了 SQLFlow 的基础环境和启动脚本，但不包含运行
    所需的 Docker 镜像，首次启动的时候需要拉取镜像。
    ```bash
    wget -c http://cdn.sqlflow.tech/latest/SQLFlowPlaygroundBare.ova
    ```
    * 完整版（2G)，这个版本包含 SQLFlow 所需的所有依赖，只需下载一次即可运行
    ```bash
    wget -c http://cdn.sqlflow.tech/latest/SQLFlowPlaygroundFull.ova
    ```
    若未安装 `wget`，也可直接复制上述链接到浏览器下载相应镜像。

1. 启动 VirtualBox，点击”管理->导入虚拟机“菜单，选择下载的 `.ova` 文件，将下载好的镜像导入到
    VirtualBox 中，双击导入的镜像启动虚拟机。
1. 通过以下命令登录虚拟机，登录后你将看到一个 start.bash 脚本，运行它即可启动 SQLFlow 系统。
    你可以在 shell 中运行 ssh 来登录，也可以直接在 VirtualBox 的虚拟机窗口中输入账号密码来登录。
    ```
    ssh -p2222 root@127.0.0.1
    root@127.0.0.1's password: sqlflow
    ./start.bash
    ```
1. 运行启动脚本后会看到一系列日志，当出现 `Access Jupyter NoteBook at: http://127.0.0.1:8888/...`
    类似的提示时，表示系统已经启动，此时在浏览器上输入该链接，即可访问到 SQLFlow 用户界面，你可以选择
    其中的例子来运行。

## 常见问题

1. Docker 镜像下载速度慢
    建议更换 Docker 镜像源，或者下载完整版（运行时无需再下载镜像）
1. start.bash 脚本异常退出
    通常是由于下载 Docker 镜像超时引起，重新启动该脚本即可
1. 脚本正常退出，但无法访问 web 界面
    通常由于容器尚在启动中，重新运行 start.bash 即可