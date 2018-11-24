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

	// Wait for messages
	for {
		msg, _ := socket.Recv(0)
		println("Received ", string(msg))

		// do some fake "work"
		time.Sleep(time.Second)

		// send reply back to client
		socket.Send("World", 0)
	}
}
