# Go Example of ZMQ Programming

## Build 

In this example, we build everything into a Docker image, whose base image is the official Jupyter image `jupyter/minimal-notebook`, so we could verify that our code uses the ZeroMQ used by Jupyter Notebook.

After git clone the repo, run `cd` to this directory, and build the Docker image using the following command:

```bash
docker build -t zmq .
```

## Run

We can run the Docker image, whose entrypoint will start the example server in the background and the example client in the foreground.  So we should see the client prints "Sending ..." and the server prints "Received ..." alternatively as follows:

```
yi@WangYis-iMac:~/go/src/github.com/wangkuiyi/sqlflow/zmq/example (magic_cmd)*$ docker run --rm -it zmq 
Connecting to hello world server…Sending  Hello 0
Received  Hello 0
Received  World
Sending  Hello 1
Received  Hello 1
Received  World
Sending  Hello 2
Received  Hello 2
Received  World
Sending  Hello 3
Received  Hello 3
^Csignal: interrupt
signal: interrupt
```
