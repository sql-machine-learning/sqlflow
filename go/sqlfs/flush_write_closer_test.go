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

package sqlfs

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func flushToBytes(output *bytes.Buffer) func([]byte) error {
	return func(buf []byte) error {
		_, e := output.Write(buf)
		return e
	}
}

func TestSQLFSWriteWithSmallBuffer(t *testing.T) {
	a := assert.New(t)
	var buf bytes.Buffer
	f := newFlushWriteCloser(flushToBytes(&buf), noopWrapUp, 1)
	f.Write([]byte("Hello World!"))
	a.NoError(f.Close())
	a.Equal("Hello World!", buf.String())
}

func TestSQLFSWriteNothing(t *testing.T) {
	a := assert.New(t)
	var buf bytes.Buffer
	f := newFlushWriteCloser(flushToBytes(&buf), noopWrapUp, 1)
	f.Write(nil)
	f.Write([]byte(""))
	a.NoError(f.Close())
	a.Equal("", buf.String())
}

func TestSQLFSWriteRecordSizeEqualToBufferSize(t *testing.T) {
	a := assert.New(t)
	var buf bytes.Buffer
	f := newFlushWriteCloser(flushToBytes(&buf), noopWrapUp, 1)
	s := "Hello World!"
	for _, c := range s {
		f.Write([]byte(fmt.Sprintf("%c", c)))
	}
	a.NoError(f.Close())
	a.Equal("Hello World!", buf.String())
}
