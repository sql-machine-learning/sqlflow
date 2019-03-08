# Closing the producer goroutine from the consumer

The producer-and-consumer pattern is well used in Go concurrent programming. When
the consumer stops, we want to gracefully stop the producer as well.

## Problem

When a gRPC server receives a streaming request,  it usually calls a
[function that returns a channel](https://talks.golang.org/2012/concurrency.slide#25),
reads the result from that channel and send the result to the client one by one.

Take the following code for instance: upon receiving a request, the main goroutine
`Service` calls `launchJob`. `launchJob` starts a separate goroutine as an anonymous
function call and returns a channel. In the anonymous function, items will be sent to
channel. And `Service` on the otherside of the channel will reads from it.

```go
func Service(req *Request, stream *StreamResponse) error {
  result := launchJob(req.Content)
  for r := range result {
    if e := stream.Send(result); e != nil {
      // should we signal the running goroutine so it will stop sending?
      return e
    }
  }
}

func launchJob(content string) chan Item {
  c := make(chan Item)
  
  go func() {
    defer close(c)

    acquireScarceResources()
    defer releaseScarceResources()

    ...
    // if stream.Send(result) returns an error and the Service returns, this will be blocked
    c <- Item{}
    ...
  }()
  
  return c
}
```

There is a major problem in this implementation. As pointed out by the comment,
if the `Send` in `Service` returns an error, the `Service` function will return,
leaving the anonymous function being blocked on `c <- Item{}` forever.

This problem is important because the leaking goroutine usually owns scarce system
resources such as network connection and memory.


## Solution: pipeline explicit cancellation

Inspired by this [blog post](https://blog.golang.org/pipelines) section
*Explicit cancellation*, we can signal the cancellation via closing on a separate
channel. And we can follow the terminology as `io.Pipe`.

```go
package sql

import (
	"errors"
)

var ErrClosedPipe = errors.New("pipe: write on closed pipe")

// pipe follows the design at https://blog.golang.org/pipelines
// - wrCh: chan for piping data
// - done: chan for signaling Close from Reader to Writer
type pipe struct {
	wrCh chan interface{}
	done chan struct{}
}

// PipeReader reads real data
type PipeReader struct {
	p *pipe
}

// PipeWriter writes real data
type PipeWriter struct {
	p *pipe
}

// Pipe creates a synchronous in-memory pipe.
//
// It is safe to call Read and Write in parallel with each other or with Close.
// Parallel calls to Read and parallel calls to Write are also safe:
// the individual calls will be gated sequentially.
func Pipe() (*PipeReader, *PipeWriter) {
	p := &pipe{
		wrCh: make(chan interface{}),
		done: make(chan struct{})}
	return &PipeReader{p}, &PipeWriter{p}
}

// Close closes the reader; subsequent writes to the
func (r *PipeReader) Close() {
	close(r.p.done)
}

// ReadAll returns the data chan. The caller should
// use it as `for r := range pr.ReadAll()`
func (r *PipeReader) ReadAll() chan interface{} {
	return r.p.wrCh
}

// Close closes the writer; subsequent ReadAll from the
// read half of the pipe will return a closed channel.
func (w *PipeWriter) Close() {
	close(w.p.wrCh)
}

// Write writes the item to the underlying data stream.
// It returns ErrClosedPipe when the data stream is closed.
func (w *PipeWriter) Write(item interface{}) error {
	select {
	case w.p.wrCh <- item:
		return nil
	case <-w.p.done:
		return ErrClosedPipe
	}
}
```

And the consumer and producer be can implemented as

```go
func Service(req *Request, stream *StreamResponse) error {
  pr := launchJob(req.Content)
  defer pr.Close()
  for r := range pr.ReadAll() {
    if e := stream.Send(r); e != nil {
      return e
    }
  }
}

func launchJob(content string) PipeReader {
	pr, pw := Pipe()
	go func() {
		defer pw.Close()
		
		if err := pw.Write(Item{}); err != nil {
			return
		}
	}
	return pr
}
```

## Further Reading

1. [Google Form: Channel send timeout](https://groups.google.com/forum/#!topic/golang-nuts/Oth9CmJPoqo)
2. [Go by Example: Timeouts](https://gobyexample.com/timeouts)
3. [Google I/O 2013 - Advanced Go Concurrency Patterns](https://www.youtube.com/watch?v=QDDwwePbDtw&t=111s)
4. [Go Concurrency Patterns Talk](https://talks.golang.org/2012/concurrency.slide)
5. [Go Concurrency Patterns: Pipelines and cancellation](https://blog.golang.org/pipelines)