# SQLFlow 的 CI 实践：从 Travis CI 切换到阿里云 ECS 和 GitHub Actions

任何软件的开发者都希望 CI 执行速度快，而且多个 CI jobs 可以并发执行。最重要的是，
不要超出预算。 SQLFlow 项目也一样。只是复杂软件系统的 CI 执行很慢，并发很费钱。
本文介绍 SQLFlow 项目的核心贡献者武毅老师组织的一次 CI 升级 —— 从 Travis CI 切换
到 GitHub Actions 和阿里云 ECS 的组合。从价格、稳定性、和功能三个角度衡量，这是
我们目前发现的最优选择。很多大型软件项目的 CI 配置的复杂度更甚于 SQLFlow，尤其是
以 Kubernetes 为运行环境的项目，希望我们的探索能给启发大家更好地解决复杂系统的
CI 问题。

## SQLFlow 的 CI 需求

SQLFlow 是一个编译器；它把扩展语法的SQL程序翻译成 AI workflow，在Kubernetes 机群
上执行。SQLFlow 支持多种数据库系统，包括 MySQL、Apache Hive、阿里巴巴 MaxCompute，
以及多种AI模式，包括用 TenosrFlow 和 PyTorch 实现的深度学习模型、以及基于
XGBoost 的树模型，包块有监督和无监督学习、模型的可视化解释、甚至运筹规划。这就意
味着，一个 SQLFlow 生成的 workflow 应该可以用任何上述数据库系统里的数据来训练任
何上述模型。那么，CI worker 里需要部署这些数据库系统和 AI 系统。这导致 CI 执行速
度慢。

另一个导致 CI 执行慢的原因是 workflow 的执行需要 Kubernetes 机群；我们用
minikube 为 CI 创建微型 Kubernetes 机群，而 minikube 不能运行在启动速度快的
Docker container 里，而得运行在虚拟机里。所以 CI worker 必须是虚拟机。

既然我们离不开虚拟机，那么就希望把 Kubernetes 机群，以及运行在 Kubernetes 机群里
的数据库系统和 AI 系统的 Docker images 都预先放在虚拟机镜像里，从而 CI 时部署第
三方系统的时间。

可惜，我们之前的两年里都在用的 Travis CI 不允许使用用户定制的虚拟机镜像 —— 其实
也可以提供镜像，只是需要支付高昂的 Enterprise 服务的使用费，超出一般开源项目的预
算。这就导致每次 CI 都需要在标准虚拟机里安装 minikube，创建虚拟 Kubernetes 机群，
下载第三方软件的 Docker images，非常慢。

而且，Travis CI 的并发度取决于花钱买 workers。随着 SQLFlow 开发者的增加和代码修
改频率的加速，对并发 workers 的数量要求越来越多，已经超出了我们的预算。

## 推而广之

SQLFlow 面对的问题也并存于很多复杂的依赖众多的软件系统，尤其是依赖 Kubernetes 作
为运行环境的系统。

我们希望 CI job 运行速度快，而 CI job 里通常包括两部分工作：

1. 安装和部署 dependencies。具体包括：
   1. 通过包管理器，例如 apt 和 Homebrew，安装软件；
   1. 执行 Docker pull 下载 images；
   1. 运行一些软件，例如 minikube，创建 CI jobs 的运行环境；
   1. 下载和预处理测试数据。

1. 执行各种 unit tests 和 regression tests。

为了加速 1.，我们希望 CI jobs 运行在 Docker containers 里，而不是虚拟机里，因为
Docker containers 的启动速度更快。但是，Docker runtime 对其中运行的程序的隔离不
彻底，可能让其中的程序的恶意行为破坏整个 CI 服务。所以很多服务包括 Travis CI 近
年来取消了 Docker container 作为 CI worker 的选项。另外，上文提到，运行
Kubernetes 也需要 CI worker 是虚拟机，可见复杂系统的 CI 对虚拟机的需求难免。

既然只能用虚拟机作为 CI worker，那么我们就希望预装软件到虚拟机镜像里，同时希望
CI 服务能让我们用自己定制的镜像。可惜常用 CI 服务要么不支持、要么太贵。为此，本
文介绍的方法支持使用定制镜像。

为了加速 2.，我们希望把 tests 分成互不依赖的多组，这样组之间可以并发，从而利用更
多的 workers 来加速每次代码修改触发的 CI 工作。这里有两个前提：

1. CI worker 的准备时间足够快，不要准备时间比执行一组 tests 还长。这就回到上上述
   分析 —— 需要 CI 系统能执行用户定制的虚拟机镜像。
1. 便宜 —— 不要因为并发执行多个 CI workers 超出预算。

本文介绍的方法利用阿里云 ECS 提升性价比。

## 基于 Travis CI 的性能优化

在使用 Travis CI 的两年里，我们在 Travis CI 允许的定制范围里尽力优化了 CI 性能。

比如，为了省去下载测试数据的时间，我们基于数据库系统的 Docker image（比如 MySQL
image）定制自己的 Docker image，在里面预先创建好数据库表格，并且把数据加入进去。

再比如，为了加速 CI 过程中下载第三方软件的 Docker images，我们不是从
Dockerhub.com 下载，而是预先把 Docker images 转换成文件的形式，放在 Travis CI 提
供的 cache 里。但是每次运行 CI 的时候，把文件转成 Docker image 也是要耗时的。另
外，我们也尝试过在 AWS 上搭建自己的 Docker registry —— 因为 Travis CI 用的是 AWS；
不过 Dockerhub.com 也是用的 AWS，所以并不是总是比从 Dockerhub.com 下载更快。

其实最快的方式是 Docker image 预先下载好，放在 CI 的虚拟机镜像里 —— 这样每次
Travis CI 启动虚拟机来运行 CI jobs 的时候，运行我们的虚拟机镜像。可惜 Travis CI
支持定制的虚拟机镜像的收费太贵，我们只能使用标准镜像。

即使不能定制虚拟机镜像，我们还是努力尝试定制 Docker images。比如，我们把 SQLFlow
的源码和 SQLFlow CI 依赖的第三方软件（例如 MySQL）和测试数据分别封装在不同的
Docker images 里。这样，每次 CI 的时候，只需要重新构建 SQLFlow 的 Docker image，
而不需要修改包括 MySQL 和测试数据的 image。即使如此，每次 CI 的时候仍然需要下载
这些 Docker images，我们仍然希望支持定制虚拟机镜像，从而可以预先下载 Docker
images 到虚拟机镜像里。

## 使用 GitHub Actions 和阿里云 ECS

我们的方案是自己在阿里云上利用 ECS 服务创建虚拟机 CI workers。ECS 允许我们定制虚
拟机镜像。其他公有云服务也支持，阿里云的性价比更高。

我们的做法是先创建一个 ECS 虚拟机，在里面配置要 CI worker 的执行环境。然后把这个
虚拟机打包成一个镜像，然后启动更多 CI workers，都执行这个镜像。

### 创建第一个 ECS CI Worker

首先我们登录阿里云控制台，创建一台 ECS 虚拟机。一个小技巧是选择地域为香港，这个
地域的节点访问美国的服务，例如 DockerHub，速度很快；而且 CI 发布的预编译好的软件
包被国内用户访问的速度也很快。机器配置应该根据 CI 任务的负载选择。SQLFlow 项目用
的是 4vCPU 8GiB的机型。

虚拟机运行起来之后，我们部署编译环境和运行环境。对于 SQLFlow 项目，可以参考
docker/dev/install.sh 和 scripts/travis/install_minikube.sh 两个脚本。为了验证当
前机器的环境可以正确的执行 SQLFlow 的所有单元测试任务，我们先在当前机器上手动运
行单测，以便补充安装需要的工具。

### 注册 GitHub Actions Worker

接下来，我们把当前的虚拟机注册为 Github Actions 的一个 CI worker。在 Github
SQLFlow 项目的 Settings 页面（只有管理员权限才可以查看修改），点击左侧的 Actions
标签，会进入 Github Actions 设置页面，点击 Add runner 按钮，会出现一个解释如何部
署 worker 的文档，在这个页面中选择 Operating System: Linux, Architecture: x64，
对应 ECS 的操作系统和 CPU 类型。然后复制下面的安装命令即可在把当前 ECS 注册成为
SQLFlow 项目的 worker。注册成功的 worker，可以在 Settings -> Actions 页面下显示
在列表中。

![注册workers](https://pic3.zhimg.com/80/v2-f0f1b1405aba13676c94fa5e5278b8d2_1440w.jpg)

### 保存虚拟机镜像和复制更多 Workers

完成一台 ECS 的配置之后，我们可以在阿里云控制台中将当前 ECS 保存一个镜像，再启动
更多的机器，增加更多的并发 worker。通过镜像启动的新的 worker 只需要简单配置新
worker 的 hostname，`/etc/hosts`即可，注册为新的 Github Actions worker。

### 配置 GitHub Actions 工作流程

创建好 CI workers 之后，我们就可以开始配置 Github Actions的 CI 任务的工作流程了。
为此，我们需要写一个 YAML 文件，放在源码目录树中。其编写方式请参考 [GitHub
Actions 的文
档](https://docs.github.com/en/free-pro-team@latest/actions/quickstart)。SQLFlow
的配置在[这
里
](https://github.com/sql-machine-learning/sqlflow/blob/develop/.github/workflows/main.yml)
，其中包括 CI 和 CD 两部分：

1. CI 部分负责运行测试，包括可以交给不同的 CI workers 并发执行的 jobs，例如
   test-mysql、test-hive-java、test-workflow 等。可以看到这些 job 都标记了
   `runs-on: [self-hosted, linux]`，让 GitHub Actions 把这些 jobs 运行在
   "self-hosted" 的机器上，也就是我们自定以的 ECS 的虚拟机上。

1. CD 部分在 CI 成功后才执行，负责发布 CI 部分编译得到的 SQLFlow 的客户端程序以
   及 Docker images。所以其中的 jobs 包括： push-images, linux-client,
   macos-client, windows-client。这些 jobs 不需要执行单测，所以没有用 ECS 虚拟机，
   而是用 Github Actions 提供的的公用 worker 执行。我们可以看到标记为 `runs-on:
   ubuntu-latest`。

### 设置和使用保密信息

因为 CD 要发布 Docker images，所以需要 DockerHub.com 的账户信息。类似的保密信息
还有很多，比如 CI 中为了执行 SQLFlow 编译生成的 workflows，需要能访问阿里云上的
MaxCompute 数据库。我们不希望这些敏感的信息因为放在 YAML 文件里所以被公开，为此我们在
Github Actions 的配置页面里输入这些信息，在 YAML 文件里只是引用这些信息。

1. 在 Github Settings -> Secrets 中添加保密信息，点击 "New secret" 按钮，填写信
   息的 key （例如：SECRET_KEY） 和 secret value。
1. 在 Github Actions YAML 配置文件中的 env 设置部分，指定 `SECRET_KEY: ${{
   secrets.SECRET_KEY }}`，后续使用环境变量 $SECRET_KEY 就可以得到安全的保密信息。

![输入secrets](https://pic2.zhimg.com/80/v2-fabc55bae4b5dfc832f4e889d95cfd09_1440w.jpg)

### 清理 ECS 垃圾文件

在上述 CI jobs 里，运行测试依赖的数据库系统都运行在 Docker container 里。当 CI
job 结束的时候，这些 containers 就被删掉了，所以中间数据处理结果不会在 ECS 虚拟
机上累积。

不过，有一些数据是需要专门清除的。比如，执行 unit tests 调用 SQLFlow 编译器产生
的 workflow 时，会在 Kubernetes 机群里创建一些 pods。我们需要定时清理掉这些内容
保证 ECS 的磁盘空间不会一直增长。为此，我们在每台 ECS 机器上配置了 crontab，定时
清理过期的 Kubernetes objects 以及 Docker images。

## 后记

Github Actions 和阿里云 ECS 的组合允许我们定制 CI worker 的虚拟机镜像，从而最大
限度的复用 CI 需要部署的第三方软件。SQLFlow 使用此方式把 CI 时间从一小时缩减到30
分钟以内。Github Actions 也提供有限的免费 CI worker 资源，用于小规模开源项目。
