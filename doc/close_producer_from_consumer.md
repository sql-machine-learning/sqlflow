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

## Solution

The proposed solution contains a new data structure `Subscription`.

```go
struct Subscription {
  updates: chan interface{}
  closing: chan bool
}

func (s *Subscription) Updates() chan interface{} {
  return s.updates
}

func (s *Subscription) Close() {
  s.closing <- true
}

func (s *Subscription) Send(item interface{}) error {
  select {
  case s.updates <- item:
    return nil // successfully send 
  case <-s.closing:
    return fmt.Errorf("Closing channel")
  }
}
```

And it can be used in the following way

```go
func Service(req *Request, stream *StreamResponse) error {
  s := launchJob(req.Content)
  for r := range s.Updates() {
    if e := stream.Send(result); e != nil {
      s.Close()
      return e
    }
  }
}

func launchJob(content string) *Subscription {
  s := &Subscription{updates: make(chan interface{}),
                     closing: make(chan bool, 1)}
  
  go func() error  {
    defer close(s.updates)

    acquireScarceResources()
    defer releaseScarceResources()

    if err := s.Send(Item{}); err != nil {
      return err
    }
    if err := s.Send(Item{}); err != nil {
      return err
    }
    ...
  }()
  
  return s
}
```

## Further Reading

1. [Google Form: Channel send timeout](https://groups.google.com/forum/#!topic/golang-nuts/Oth9CmJPoqo)
2. [Go by Example: Timeouts](https://gobyexample.com/timeouts)
3. [Google I/O 2013 - Advanced Go Concurrency Patterns](https://www.youtube.com/watch?v=QDDwwePbDtw&t=111s)
4. [Go Concurrency Patterns](https://talks.golang.org/2012/concurrency.slide)