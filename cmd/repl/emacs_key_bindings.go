// Copyright 2019 The SQLFlow Authors. All rights reserved.
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

import prompt "github.com/c-bata/go-prompt"

var emacsMetaKeyBindings = []prompt.ASCIICodeBind{
	// Meta b/B/<-: Move cursor left by word.
	{
		ASCIICode: []byte{0x1b, 'b'},
		Fn: func(buf *prompt.Buffer) {
			prompt.GoLeftWord(buf)
		},
	},

	{
		ASCIICode: []byte{0x1b, 'B'},
		Fn: func(buf *prompt.Buffer) {
			prompt.GoLeftWord(buf)
		},
	},

	{
		ASCIICode: []byte{0x1b, 0x1b, 0x5b, 0x44},
		Fn: func(buf *prompt.Buffer) {
			prompt.GoLeftWord(buf)
		},
	},

	// Meta f/F/->: Move cursor right by word.
	{
		ASCIICode: []byte{0x1b, 'f'},
		Fn: func(buf *prompt.Buffer) {
			prompt.GoRightWord(buf)
		},
	},

	{
		ASCIICode: []byte{0x1b, 'F'},
		Fn: func(buf *prompt.Buffer) {
			prompt.GoRightWord(buf)
		},
	},

	{
		ASCIICode: []byte{0x1b, 0x1b, 0x5b, 0x43},
		Fn: func(buf *prompt.Buffer) {
			prompt.GoLeftWord(buf)
		},
	},

	// Meta d/D: Delete word after the cursor
	{
		ASCIICode: []byte{0x1b, 'd'},
		Fn: func(buf *prompt.Buffer) {
			pos1 := buf.DisplayCursorPosition()
			prompt.GoRightWord(buf)
			pos2 := buf.DisplayCursorPosition()
			buf.DeleteBeforeCursor(pos2 - pos1)
		},
	},

	{
		ASCIICode: []byte{0x1b, 'D'},
		Fn: func(buf *prompt.Buffer) {
			pos1 := buf.DisplayCursorPosition()
			prompt.GoRightWord(buf)
			pos2 := buf.DisplayCursorPosition()
			buf.DeleteBeforeCursor(pos2 - pos1)
		},
	},

	// Meta Backspace: Delete word before the cursor
	{
		ASCIICode: []byte{0x1b, 0x7f},
		Fn: func(buf *prompt.Buffer) {
			prompt.DeleteWord(buf)
		},
	},
}
