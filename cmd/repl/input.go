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

package main

import (
	prompt "github.com/c-bata/go-prompt"
)

const maxReadBytes = 1024

// SQLFlowConsoleParser is a ConsoleParser implementation for POSIX environment.
type stdinParser struct {
	prompt.ConsoleParser
	buf    []byte
	start  int
	size   int
	keyMap map[prompt.Key][]byte
}

// Merge similar keys
func defaultKeyMap() map[prompt.Key][]byte {
	return map[prompt.Key][]byte{
		prompt.Up:       []byte{0x1b, 'p'},
		prompt.ControlP: []byte{0x1b, 'p'},
		prompt.Down:     []byte{0x1b, 'n'},
		prompt.ControlN: []byte{0x1b, 'n'},
	}
}

// Read returns byte array.
func (p *stdinParser) Read() ([]byte, error) {
	if p.size == p.start {
		buf, err := p.ConsoleParser.Read()
		if err != nil {
			return []byte{}, err
		}
		p.start = 0
		p.size = len(buf)
		p.buf = buf
	}
	i := p.start
L:
	for ; i < p.size; i++ {
		if remapped, ok := p.keyMap[prompt.GetKey(p.buf[i:])]; ok {
			p.start = p.size
			return remapped, nil
		}
		switch prompt.GetKey(p.buf[i : i+1]) {
		case prompt.Enter, prompt.ControlJ, prompt.ControlM:
			break L
		}
	}
	if i == p.start {
		p.start++
		return p.buf[i : i+1], nil
	}
	buf := p.buf[p.start:i]
	p.start = i
	return buf, nil
}

func (p *stdinParser) defineKeyMap(key prompt.Key, remapped []byte) {
	p.keyMap[key] = remapped
}

func (p *stdinParser) resetKeyMap() {
	p.keyMap = defaultKeyMap()
}

// newStdinParser returns ConsoleParser object to read from stdin.
func newStdinParser() *stdinParser {
	return &stdinParser{
		ConsoleParser: prompt.NewStandardInputParser(),
		keyMap:        defaultKeyMap(),
	}
}
