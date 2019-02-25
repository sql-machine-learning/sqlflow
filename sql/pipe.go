package sql

import "fmt"

type pipe struct {
	wrCh chan interface{}
	done chan bool
}

type PipeReader struct {
	p *pipe
}
type PipeWriter struct {
	p *pipe
}

// Pipe: Creates a pipe
// - wrCh: chan for storing data
// - done: Reader closes pipe if error happens, then Writer encounters an error
func Pipe() (*PipeReader, *PipeWriter) {
	p := &pipe{
		wrCh: make(chan interface{}),
		done: make(chan bool, 1)}
	return &PipeReader{p}, &PipeWriter{p}
}

// Close: then no more data could be written
func (r *PipeReader) Close() {
	r.p.done <- true
}

// ReadAll: returns the data chan
func (r *PipeReader) ReadAll() chan interface{} {
	return r.p.wrCh
}

// Close: ends the writer
func (w *PipeWriter) Close() {
	close(w.p.wrCh)
}

// Write: returns an error after close writer
func (w *PipeWriter) Write(item interface{}) error {
	select {
	case w.p.wrCh <- item:
		return nil
	case <-w.p.done:
		return fmt.Errorf("pipe closed already")
	}
}
