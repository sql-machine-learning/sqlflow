// Copyright 2020 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pipe

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
