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
	"github.com/c-bata/go-prompt"
	"strings"
)

// stdoutWriter is a prompt.ConsoleWriter implementation for SQLFlow.
type stdoutWriter struct {
	prompt.ConsoleWriter
	currentBgColor prompt.Color
	currentFgColor prompt.Color
}

// TODO(shendiaomo): get a full list of SQL keywords according to the dialect
var keywords = map[string]bool{
	"ALTER":     true,
	"COLUMN":    true,
	"CREATE":    true,
	"DATABASE":  true,
	"DATABASES": true,
	"DESC":      true,
	"DESCRIBE":  true,
	"EXPLAIN":   true,
	"FROM":      true,
	"INSERT":    true,
	"INTO":      true,
	"LIMIT":     true,
	"SELECT":    true,
	"SHOW":      true,
	"TABLE":     true,
	"TABLES":    true,
	"TO":        true,
	"UPDATE":    true,
	"USE":       true,
	"USING":     true,
	"VALUES":    true,
	"WHERE":     true,
	"WITH":      true,
}
var extendedKeywords = map[string]bool{
	"TRAIN":   true,
	"PREDICT": true,
	"LABEL":   true,
}

// newStdoutWriter returns a ConsoleWriter object to write to stdout.
func newStdoutWriter() *stdoutWriter {
	return &stdoutWriter{ConsoleWriter: prompt.NewStdoutWriter()}
}

// WriteStr writes safety string by removing control sequences and highlighting keywords.
func (w *stdoutWriter) WriteStr(data string) {
	words := strings.Split(data, " ")
	for i, word := range words {
		if w.isInputLine() {
			if _, ok := keywords[strings.ToUpper(word)]; ok {
				w.ConsoleWriter.SetColor(prompt.Cyan, prompt.DefaultColor, true)
			} else if _, ok := extendedKeywords[strings.ToUpper(word)]; ok {
				w.ConsoleWriter.SetColor(prompt.DarkBlue, prompt.DefaultColor, true)
			}
			w.ConsoleWriter.Write([]byte(word))
			w.ConsoleWriter.SetColor(prompt.DefaultColor, prompt.DefaultColor, false)
		} else {
			w.ConsoleWriter.Write([]byte(word))
		}
		if i != len(words)-1 {
			w.ConsoleWriter.Write([]byte(" "))
		}
	}

	return
}

// SetColor sets text and background colors. and specify whether text is bold.
func (w *stdoutWriter) SetColor(fg, bg prompt.Color, bold bool) {
	w.ConsoleWriter.SetColor(fg, bg, bold)
	w.currentFgColor = fg
	w.currentBgColor = bg
	return
}

// isInputLine checks whether we're rendering the input line
func (w *stdoutWriter) isInputLine() bool {
	// TODO(shendiaomo): try to find another way to check this
	if w.currentBgColor == prompt.DefaultColor && w.currentFgColor == prompt.DefaultColor {
		return true
	}
	return false
}
