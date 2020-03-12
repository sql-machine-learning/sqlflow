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
	"log"

	"github.com/c-bata/go-prompt"
)

var clipboard string // This is not the system clipboard and only works in SQLFlow REPL
const wordSeparator = ",.=() "

func deleteWord(buf *prompt.Buffer) {
	buf.DeleteBeforeCursor(len([]rune(buf.Document().TextBeforeCursor())) - buf.Document().FindStartOfPreviousWordUntilSeparatorIgnoreNextToCursor(wordSeparator))
}

func goRightWord(buf *prompt.Buffer) {
	buf.CursorRight(buf.Document().FindEndOfCurrentWordUntilSeparatorIgnoreNextToCursor(wordSeparator))
}

func goLeftWord(buf *prompt.Buffer) {
	buf.CursorLeft(len([]rune(buf.Document().TextBeforeCursor())) - buf.Document().FindStartOfPreviousWordUntilSeparatorIgnoreNextToCursor(wordSeparator))
}

var emacsMetaKeyBindings = []prompt.ASCIICodeBind{
	// Meta b/B/<-: Move cursor left by word.
	{
		ASCIICode: []byte{0x1b, 'b'},
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			goLeftWord(buf)
		},
	},
	{
		ASCIICode: []byte{0x1b, 'B'},
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			goLeftWord(buf)
		},
	},
	{
		ASCIICode: []byte{0x1b, 0x1b, 0x5b, 0x44},
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			goLeftWord(buf)
		},
	},
	// Meta f/F/->: Move cursor right by word.
	{
		ASCIICode: []byte{0x1b, 'f'},
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			goRightWord(buf)
		},
	},
	{
		ASCIICode: []byte{0x1b, 'F'},
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			goRightWord(buf)
		},
	},
	{
		ASCIICode: []byte{0x1b, 0x1b, 0x5b, 0x43},
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			goRightWord(buf)
		},
	},
	// Meta d/D: Cut the word after the cursor to the clipboard
	{
		ASCIICode: []byte{0x1b, 'd'},
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			pos1 := buf.DisplayCursorPosition()
			goRightWord(buf)
			pos2 := buf.DisplayCursorPosition()
			clipboard = buf.DeleteBeforeCursor(pos2 - pos1)
		},
	},
	{
		ASCIICode: []byte{0x1b, 'D'},
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			pos1 := buf.DisplayCursorPosition()
			goRightWord(buf)
			pos2 := buf.DisplayCursorPosition()
			clipboard = buf.DeleteBeforeCursor(pos2 - pos1)
		},
	},
	// Meta Backspace: Delete word before the cursor
	{
		ASCIICode: []byte{0x1b, 0x7f},
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			clipboard = buf.DeleteBeforeCursor(len([]rune(buf.Document().GetWordBeforeCursorUntilSeparatorIgnoreNextToCursor(wordSeparator))))
		},
	},
	// Meta P: Navigate the older command
	{
		ASCIICode: []byte{0x1b, 'p'},
		Fn: func(buf *prompt.Buffer) {
			navigateHistory(buf, true)
		},
	},
	{
		ASCIICode: []byte{0x1b, 'P'},
		Fn: func(buf *prompt.Buffer) {
			navigateHistory(buf, true)
		},
	},

	// Meta N: Navigate the newer command
	{
		ASCIICode: []byte{0x1b, 'n'},
		Fn: func(buf *prompt.Buffer) {
			navigateHistory(buf, false)
		},
	},
	{
		ASCIICode: []byte{0x1b, 'N'},
		Fn: func(buf *prompt.Buffer) {
			navigateHistory(buf, false)
		},
	},
	// Meta W: Search command history in wildcard
	{
		ASCIICode: []byte{0x1b, 'w'},
		Fn: func(buf *prompt.Buffer) {
			startSearch(buf, wildcardSearch)
		},
	},
	{
		ASCIICode: []byte{0x1b, 'W'},
		Fn: func(buf *prompt.Buffer) {
			startSearch(buf, wildcardSearch)
		},
	},
}

var emacsCtrlKeyBindings = []prompt.KeyBind{
	// Go to the End of the line
	{
		Key: prompt.ControlE,
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			x := []rune(buf.Document().TextAfterCursor())
			buf.CursorRight(len(x))
		},
	},
	// Go to the beginning of the line
	{
		Key: prompt.ControlA,
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			x := []rune(buf.Document().TextBeforeCursor())
			buf.CursorLeft(len(x))
		},
	},
	// Cut the Line after the cursor to the clipboard
	{
		Key: prompt.ControlK,
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			x := []rune(buf.Document().TextAfterCursor())
			clipboard = buf.Delete(len(x))
		},
	},
	// Cut the Line before the cursor to the clipboard
	{
		Key: prompt.ControlU,
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			x := []rune(buf.Document().TextBeforeCursor())
			clipboard = buf.DeleteBeforeCursor(len(x))
		},
	},
	// Delete the character under the cursor
	{
		Key: prompt.ControlD,
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			if buf.Text() != "" {
				buf.Delete(1)
			}
		},
	},
	// Same as backspace
	{
		Key: prompt.ControlH,
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			buf.DeleteBeforeCursor(1)
		},
	},

	// Right arrow: Go forward one character
	{
		Key: prompt.ControlF,
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			buf.CursorRight(1)
		},
	},
	// Left arrow: Go backward one character
	{
		Key: prompt.ControlB,
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			buf.CursorLeft(1)
		},
	},
	// Cut the Word before the cursor to the clipboard
	{
		Key: prompt.ControlW,
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
			clipboard = buf.DeleteBeforeCursor(len([]rune(buf.Document().GetWordBeforeCursorUntilSeparatorIgnoreNextToCursor(wordSeparator))))
		},
	},
	// Clear the Screen, similar to the clear command
	{
		Key: prompt.ControlL,
		Fn: func(buf *prompt.Buffer) {
			stopSearch("")
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
			stopSearch("")
			buf.InsertText(clipboard, false, true)
		},
	},
	// Search command history
	{
		Key: prompt.ControlR,
		Fn: func(buf *prompt.Buffer) {
			startSearch(buf, normalSearch)
		},
	},
}

var startSearch = func(*prompt.Buffer, searchMode) {}
var stopSearch = func(string) (selected string, totalLen int) { return }
var navigateHistory = func(*prompt.Buffer, bool) {}
