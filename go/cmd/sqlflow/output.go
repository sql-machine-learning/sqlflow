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
	"strings"

	"github.com/alecthomas/chroma/quick"
	"github.com/c-bata/go-prompt"
)

// stdoutWriter is a prompt.ConsoleWriter implementation for SQLFlow.
type stdoutWriter struct {
	prompt.ConsoleWriter
	currentBgColor prompt.Color
	currentFgColor prompt.Color
}

var extendedKeywords = map[string]bool{
	"COLUMN":   true,
	"USING":    true,
	"EXPLAIN":  true,
	"EVALUATE": true,
	"LABEL":    true,
	"PREDICT":  true,
	"TO":       true,
	"TRAIN":    true,
	"WITH":     true,
}

// newStdoutWriter returns a ConsoleWriter object to write to stdout.
func newStdoutWriter() *stdoutWriter {
	return &stdoutWriter{ConsoleWriter: prompt.NewStdoutWriter()}
}

// WriteStr writes safety string by removing control sequences and highlighting keywords.
func (w *stdoutWriter) WriteStr(data string) {
	// TODO(shendiaomo): remove control sequences
	var buf strings.Builder
	if w.isInputLine() && data != prefix && data != livePrefix {
		// TODO(shendiaomo): switch SQL keywords according to the dialect
		// FIXME(shendiaomo): try highlighting strings across multiple lines
		quick.Highlight(&buf, data, "mysql", "terminal16m", "monokai")
		words := strings.Split(buf.String(), " ")
		cleanWords := strings.Split(data, " ")
		if len(words) != len(cleanWords) {
			w.ConsoleWriter.WriteRaw([]byte(buf.String()))
			return
		}
		for i, word := range words {
			if _, ok := extendedKeywords[strings.ToUpper(cleanWords[i])]; ok {
				w.ConsoleWriter.SetColor(prompt.DarkGreen, prompt.DefaultColor, false)
				w.ConsoleWriter.Write([]byte(cleanWords[i]))
				w.ConsoleWriter.SetColor(prompt.DefaultColor, prompt.DefaultColor, false)
			} else {
				w.ConsoleWriter.WriteRaw([]byte(word))
			}
			if i != len(words)-1 {
				w.ConsoleWriter.Write([]byte(" "))
			}
		}
	} else {
		w.ConsoleWriter.Write([]byte(data))
	}
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
