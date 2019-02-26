package sql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPipeWriteRead(t *testing.T) {
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

func TestPipeClose(t *testing.T) {
	a := assert.New(t)
	rd, wr := Pipe()

	// writer
	go func(n int) {
		defer wr.Close()
		for i := 0; i < n; i++ {
			e := wr.Write(i)
			a.NoError(e)
			time.Sleep(100 * time.Millisecond)
		}
	}(5)

	rd.Close()
}
