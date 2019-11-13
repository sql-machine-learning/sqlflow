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

import (
	"fmt"
	"strings"

	prompt "github.com/c-bata/go-prompt"
)

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
			count := buf.Document().FindEndOfCurrentWord()
			prompt.GoRightWord(buf)
			buf.DeleteBeforeCursor(count)
		},
	},

	{
		ASCIICode: []byte{0x1b, 'D'},
		Fn: func(buf *prompt.Buffer) {
			count := buf.Document().FindEndOfCurrentWord() - len([]rune(buf.Document().TextBeforeCursor()))
			prompt.GoRightWord(buf)
			buf.DeleteBeforeCursor(count)
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

var defaultSuggestions = []prompt.Suggest{
	{"SELECT", ""},
	{"FROM", ""},
	{"WHERE", ""},
	{"LIMIT", ""},
	{"COLUMN", ""},
	{"INTO", ""},
	{"USING", ""},
	{"WITH", ""},
	{"TO", ""},
}

var toSuggestions = []prompt.Suggest{
	{"TRAIN", "train a model"},
	{"PREDICT", "predict using a trained model"},
	{"ANALYZE", "analyze using a trained model"},
}

var withSuggestions = []prompt.Suggest{
	{"LABEL", "label for supervised learning"},
	{"INTO", "where to save model"},
	{"USING", "using a trained model"},
}

var labelSuggestions = []prompt.Suggest{
	{"INTO", "where to save model"},
}

var attributes = []prompt.Suggest{
	{"train", "training attributes"},
	{"model", "model attributes"},
	{"validation.select", "validation dataset"},
}

type promptState struct {
	prefix           string
	livePrefix       string
	enableLivePrefix bool
	statement        string
	keywords         []string
}

func (p *promptState) changeLivePrefix() (string, bool) {
	return p.livePrefix, p.enableLivePrefix
}

func (p *promptState) execute(in string, cb func(string)) {
	in = strings.TrimRight(in, " \t")
	if in != "" {
		p.statement += in
		if strings.HasSuffix(in, ";") {
			p.enableLivePrefix = false
			fmt.Println()
			cb(p.statement)
			p.statement = ""
			return
		}
		p.statement += "\n"
		p.enableLivePrefix = true
	}
}

func (p *promptState) initCompleter() {
	for _, s := range defaultSuggestions {
		p.keywords = append(p.keywords, s.Text)
	}
	for _, s := range toSuggestions {
		p.keywords = append(p.keywords, s.Text)
	}
	for _, s := range withSuggestions {
		p.keywords = append(p.keywords, s.Text)
	}
}

// lookaheadKeyword return the last keyword and the last word in words
func (p *promptState) lookaheadKeyword(words []string) (string, string) {
	if l := len(words); l > 0 {
		for i := l; i > 0; i-- {
			for _, k := range p.keywords {
				if strings.ToUpper(words[i-1]) == k {
					return k, words[l-1]
				}
			}
		}
		return "", words[l-1]
	}
	return "", ""
}

func (p *promptState) clauseUnderCursor(in prompt.Document) (string, string) {
	// TODO(shendiaomo): use SQLFlow lexer to replace strings.Fields
	words := strings.Fields(p.statement + in.TextBeforeCursor())
	return p.lookaheadKeyword(words)
}

func (p *promptState) completer(in prompt.Document) []prompt.Suggest {
	w1 := in.GetWordBeforeCursor() // empty if the cursor is under whitespace
	clause, w2 := p.clauseUnderCursor(in)
	switch clause {
	case "TO":
		return prompt.FilterHasPrefix(toSuggestions, w1, true)
	case "WITH":
		if w1 == "" {
			if clause == w2 || strings.HasSuffix(w2, ",") {
				return prompt.FilterHasPrefix(attributes, w1, true)
			}
			return prompt.FilterHasPrefix(withSuggestions, w1, true)
		}
		return prompt.FilterHasPrefix(append(withSuggestions, attributes...), w1, true)
	case "LABEL":
		return prompt.FilterHasPrefix(append(labelSuggestions), w1, true)
	default:
		if w1 == "" {
			return []prompt.Suggest{} // Nothing to suggest
		}
		return prompt.FilterHasPrefix(defaultSuggestions, w1, true)
	}
}

func newPromptState() *promptState {
	s := promptState{
		prefix:     "sqlflow> ",
		livePrefix: "      -> ",
	}
	s.initCompleter()
	return &s
}

func runPrompt(cb func(string)) {
	state := newPromptState()
	p := prompt.New(
		func(in string) { state.execute(in, cb) },
		func(in prompt.Document) []prompt.Suggest { return state.completer(in) },
		prompt.OptionAddASCIICodeBind(emacsMetaKeyBindings...),
		prompt.OptionCompletionWordSeparator(" ."),
		prompt.OptionLivePrefix(func() (string, bool) { return state.changeLivePrefix() }),
		prompt.OptionParser(newStdinParser()),
		prompt.OptionPrefix(state.prefix),
		prompt.OptionPrefixTextColor(prompt.White),
		prompt.OptionTitle("SQLFlow"),
	)
	fmt.Println("Welcome to SQLFlow.  Commands end with ;")
	fmt.Println()
	p.Run()
}
