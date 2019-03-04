package sql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPipeWriteAndRead(t *testing.T) {
	a := assert.New(t)
	rd, wr := Pipe()
	loop := 10

	// writer
	go func(n int) {
		defer wr.Close()
		for i := 0; i < n; i++ {
			e := wr.Write(i)
			a.NoError(e)
			time.Sleep(5 * time.Millisecond)
		}
	}(loop)

	// reader
	i := 0
	for data := range rd.ReadAll() {
		a.Equal(data, i)
		i++
	}
	a.Equal(i, loop)
	rd.Close()
}

func TestPipeReaderClose(t *testing.T) {
	a := assert.New(t)
	rd, wr := Pipe()

	writeReturns := make(chan bool)
	// writer
	go func() {
		defer wr.Close()
		defer func() {
			writeReturns <- true
		}()
		for {
			if e := wr.Write(1); e != nil {
				return
			}
		}
	}()

	rd.Close()
	select {
	case <-writeReturns:
	case <-time.After(time.Second):
		a.True(false, "time out on writer return")
	}
}
