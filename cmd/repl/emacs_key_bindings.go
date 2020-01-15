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
	"log"
)

var clipboard string // This is not the system clipboard and only works in SQLFlow REPL

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
	// Meta d/D: Cut the word after the cursor to the clipboard
	{
		ASCIICode: []byte{0x1b, 'd'},
		Fn: func(buf *prompt.Buffer) {
			pos1 := buf.DisplayCursorPosition()
			prompt.GoRightWord(buf)
			pos2 := buf.DisplayCursorPosition()
			clipboard = buf.DeleteBeforeCursor(pos2 - pos1)
		},
	},
	{
		ASCIICode: []byte{0x1b, 'D'},
		Fn: func(buf *prompt.Buffer) {
			pos1 := buf.DisplayCursorPosition()
			prompt.GoRightWord(buf)
			pos2 := buf.DisplayCursorPosition()
			clipboard = buf.DeleteBeforeCursor(pos2 - pos1)
		},
	},
	// Meta Backspace: Delete word before the cursor
	{
		ASCIICode: []byte{0x1b, 0x7f},
		Fn: func(buf *prompt.Buffer) {
			clipboard = buf.DeleteBeforeCursor(len([]rune(buf.Document().GetWordBeforeCursorWithSpace())))
		},
	},
}

var emacsCtrlKeyBindings = []prompt.KeyBind{
	// Go to the End of the line
	{
		Key: prompt.ControlE,
		Fn: func(buf *prompt.Buffer) {
			x := []rune(buf.Document().TextAfterCursor())
			buf.CursorRight(len(x))
		},
	},
	// Go to the beginning of the line
	{
		Key: prompt.ControlA,
		Fn: func(buf *prompt.Buffer) {
			x := []rune(buf.Document().TextBeforeCursor())
			buf.CursorLeft(len(x))
		},
	},
	// Cut the Line after the cursor to the clipboard
	{
		Key: prompt.ControlK,
		Fn: func(buf *prompt.Buffer) {
			x := []rune(buf.Document().TextAfterCursor())
			clipboard = buf.Delete(len(x))
		},
	},
	// Cut the Line before the cursor to the clipboard
	{
		Key: prompt.ControlU,
		Fn: func(buf *prompt.Buffer) {
			x := []rune(buf.Document().TextBeforeCursor())
			clipboard = buf.DeleteBeforeCursor(len(x))
		},
	},
	// Delete the character under the cursor
	{
		Key: prompt.ControlD,
		Fn: func(buf *prompt.Buffer) {
			if buf.Text() != "" {
				buf.Delete(1)
			}
		},
	},
	// Same as backspace
	{
		Key: prompt.ControlH,
		Fn: func(buf *prompt.Buffer) {
			buf.DeleteBeforeCursor(1)
		},
	},
	// Right arrow: Go forward one character
	{
		Key: prompt.ControlF,
		Fn: func(buf *prompt.Buffer) {
			buf.CursorRight(1)
		},
	},
	// Left arrow: Go backward one character
	{
		Key: prompt.ControlB,
		Fn: func(buf *prompt.Buffer) {
			buf.CursorLeft(1)
		},
	},
	// Cut the Word before the cursor to the clipboard
	{
		Key: prompt.ControlW,
		Fn: func(buf *prompt.Buffer) {
			clipboard = buf.DeleteBeforeCursor(len([]rune(buf.Document().GetWordBeforeCursorWithSpace())))
		},
	},
	// Clear the Screen, similar to the clear command
	{
		Key: prompt.ControlL,
		Fn: func(buf *prompt.Buffer) {
			consoleWriter.EraseScreen()
			consoleWriter.CursorGoTo(0, 0)
			if err := consoleWriter.Flush(); err != nil {
				log.Fatal(err)
			}
		},
	},
	// Paste the last thing to be cut (yank)
	{
		Key: prompt.ControlY,
		Fn: func(buf *prompt.Buffer) {
			buf.InsertText(clipboard, false, true)
		},
	},
}
