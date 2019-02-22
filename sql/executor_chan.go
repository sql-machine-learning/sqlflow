package sql

import "fmt"

// ExecutorChan follow doc/close_producer_from_consumer.md
// It's the return value of Run
type ExecutorChan struct {
	data chan interface{}
	quit chan bool
}

// NewExecutorChan: creates channs
func NewExecutorChan() *ExecutorChan {
	return &ExecutorChan{
		data: make(chan interface{}),
		quit: make(chan bool, 1)}
}

// Destroy: closes data chan
func (ec *ExecutorChan) Destroy() {
	close(ec.data)
}

// Read: returns a data chan
func (ec *ExecutorChan) Read() chan interface{} {
	return ec.data
}

// Close: goroutine release
func (ec *ExecutorChan) Close() {
	ec.quit <- true
}

// Write: returns an error after Close()
func (ec *ExecutorChan) Write(item interface{}) error {
	select {
	case ec.data <- item:
		return nil
	case <-ec.quit:
		return fmt.Errorf("channel closed already")
	}
}
