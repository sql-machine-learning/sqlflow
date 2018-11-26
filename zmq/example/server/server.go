package main

import (
	"time"

	zmq "github.com/pebbe/zmq4"
)

func main() {
	context, _ := zmq.NewContext()
	socket, _ := context.NewSocket(zmq.REP)
	defer socket.Close()
	socket.Bind("tcp://*:5555")

	for {
		msg, _ := socket.Recv(0)
		println("Received ", string(msg))
		time.Sleep(time.Second)  // do some fake "work"
		socket.Send("World", 0)  // reply
	}
}
