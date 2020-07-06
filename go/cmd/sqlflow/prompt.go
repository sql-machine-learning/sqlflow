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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	prompt "github.com/c-bata/go-prompt"

	"sqlflow.org/sqlflow/go/attribute"
)

type searchMode int

const (
	normalSearch searchMode = iota
	wildcardSearch
)
const (
	prefix     = "sqlflow> "
	livePrefix = "      -> "
)

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
	{"EXPLAIN", "explain using a trained model"},
}

var trainSuggestions = []prompt.Suggest{
	{"WITH", "model parameters"},
	{"LABEL", "label field for supervised learning tasks"},
	{"COLUMN", "feature columns"},
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
	prefix                         string
	livePrefix                     string
	enableLivePrefix               bool
	keywords                       []string
	history                        []prompt.Suggest
	isSearching                    bool
	searchHistory                  func() []prompt.Suggest
	historyNavPos                  int
	historyFileName                string
	estimatorCls                   string
	modelParamDocs                 map[string][]prompt.Suggest
	models                         []prompt.Suggest
	optimizers                     map[string]string
	statements, lastStatements     []string
	inQuotedString, isSingleQuoted bool
}

func (p *promptState) changeLivePrefix() (string, bool) {
	return p.livePrefix, p.enableLivePrefix
}

func (p *promptState) execute(in string, cb func(string)) {
	if p.isSearching {
		totalLen := 0
		in, totalLen = stopSearch(in)
		// Refresh the prompt
		cursorUpCount := totalLen / getTerminalColumnSize()
		if totalLen%getTerminalColumnSize() != 0 {
			cursorUpCount++
		}
		consoleWriter.EraseLine()
		for ; cursorUpCount > 0; cursorUpCount-- {
			consoleWriter.CursorUp(1)
			consoleWriter.EraseLine()
		}
		consoleWriter.Flush()
		if p.enableLivePrefix {
			fmt.Println(p.livePrefix + in)
		} else {
			fmt.Println(p.prefix + in)
		}
	}
	if strings.Trim(in, " ") != "" { // TODO(shendiaomo): handle quoted string with newline
		if addLineToStmt(in, &p.inQuotedString, &p.isSingleQuoted, &p.statements) {
			p.updateHistory()
			p.enableLivePrefix = false
			for _, stmt := range p.statements {
				cb(stmt)
			}
			p.lastStatements = p.statements
			p.statements = []string{}
			p.optimizers = make(map[string]string)
			return
		}
		p.enableLivePrefix = true
	}
	p.historyNavPos = 0
}

func sortPromptSuggest(suggests []prompt.Suggest) {
	sort.Slice(suggests, func(i, j int) bool { return suggests[i].Text < suggests[j].Text })
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
	// initialize models and model parameters
	p.modelParamDocs = make(map[string][]prompt.Suggest)
	for model, params := range attribute.PremadeModelParamsDocs {
		p.modelParamDocs[model] = attributes
		p.models = append(p.models, prompt.Suggest{model, ""})
		prefix := "model."
		if strings.HasPrefix(model, "xgboost") {
			prefix = ""
		}
		for param, doc := range params {
			p.modelParamDocs[model] = append(p.modelParamDocs[model], prompt.Suggest{prefix + param, doc})
		}
		sortPromptSuggest(p.modelParamDocs[model])
	}
	sortPromptSuggest(p.models)
	p.optimizers = make(map[string]string) // Optimizers cannot be initialized beforehand
}

func (p *promptState) initHistory() {
	// Register history search key bindings
	startSearch = func(buf *prompt.Buffer, mode searchMode) { p.startSearch(buf, mode) }
	for _, key := range []prompt.Key{prompt.End, prompt.Home, prompt.Right, prompt.Left} {
		emacsCtrlKeyBindings = append(
			emacsCtrlKeyBindings,
			prompt.KeyBind{
				key,
				func(buf *prompt.Buffer) {
					stopSearch("")
				}})
	}
	origInput := ""
	navigateHistory = func(buf *prompt.Buffer, older bool) {
		if p.historyNavPos == 0 {
			origInput = buf.Text()
		}
		p.navigateHistory(origInput, older, buf)
	}
	// Load existing history file
	p.historyFileName = filepath.Join(os.Getenv("HOME"), ".sqlflow_history")
	f, err := os.Open(p.historyFileName)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		p.history = append([]prompt.Suggest{prompt.Suggest{Text: scanner.Text()}}, p.history...)
	}
}

func (p *promptState) searchHistoryImpl(mode searchMode, suffix string, buf *prompt.Buffer, key, lastKey, selected *string) []prompt.Suggest {
	*lastKey = *key
	in := strings.TrimPrefix(buf.Text(), *key+suffix)
	if in != buf.Text() {
		switch len(in) {
		case 0, 1:
		default:
			if strings.HasSuffix(in, ";") {
				*selected = in
			}
			pieces := strings.Split(in, ";")
			in = pieces[len(pieces)-1]
		}
		prompt.GoLineBeginning(buf)
		buf.Delete(len([]rune(buf.Text())))
		*key += in
		buf.InsertText(*key+suffix, false, true)
	} else { // Backspace
		if len(*key) != 0 {
			r := []rune(*key)
			*key = string(r[:len(r)-1])
			buf.DeleteBeforeCursor(len(suffix))
			buf.InsertText(suffix, false, true)
		} else {
			buf.InsertText(suffix[len(suffix)-1:], false, true)
		}
	}
	// Unique history commands
	counts := make(map[string]int)
	candidates := []prompt.Suggest{}
	switch mode {
	case wildcardSearch:
		pattern := "*" + strings.ToUpper(*key) + "*" // Match substring
		for _, entry := range p.history {
			if matched, err := filepath.Match(pattern, strings.ToUpper(entry.Text)); err == nil && matched {
				candidates = append(candidates, entry)
			}
		}
	default:
		candidates = prompt.FilterContains(p.history, *key, true)
	}
	for _, entry := range candidates {
		counts[entry.Text]++
	}
	result := []prompt.Suggest{}
	visited := make(map[string]bool)
	for _, entry := range candidates {
		if _, ok := visited[entry.Text]; !ok {
			result = append(result, prompt.Suggest{entry.Text, fmt.Sprintf("%d", counts[entry.Text])})
		}
		visited[entry.Text] = true
	}
	return result
}

func (p *promptState) startSearch(buf *prompt.Buffer, mode searchMode) {
	if p.isSearching {
		return
	}
	if consoleParser != nil {
		// Enable using Up, ControlP, Down, ControlN to navigate
		consoleParser.defineKeyMap(prompt.Down, []byte{0x9})
		consoleParser.defineKeyMap(prompt.ControlN, []byte{0x9})
		consoleParser.defineKeyMap(prompt.Up, []byte{0x1b, 0x5b, 0x5a})
		consoleParser.defineKeyMap(prompt.ControlP, []byte{0x1b, 0x5b, 0x5a})
	}
	oldLivePrefix := p.livePrefix
	oldEnableLivePrefix := p.enableLivePrefix
	p.isSearching = true
	switch mode {
	case wildcardSearch:
		p.livePrefix = "🔍❄`"
	default:
		p.livePrefix = "🔍`"
	}
	searchSuffix := "_` " // Must end with whitespace to make completion stop
	searchKey := buf.Text()
	keepOriginalInput := len(searchKey) != 0
	lastSearchKey := ""
	selected := ""
	prompt.GoLineEnd(buf) // Insert searchSuffix to the end of the line
	buf.InsertText(searchSuffix, false, true)
	p.enableLivePrefix = true
	p.searchHistory = func() []prompt.Suggest {
		return p.searchHistoryImpl(mode, searchSuffix, buf, &searchKey, &lastSearchKey, &selected)
	}
	stopSearch = func(in string) (string, int) {
		if !p.isSearching {
			return "", 0
		}
		if consoleParser != nil {
			consoleParser.resetKeyMap()
		}
		prompt.GoLineEnd(buf)
		p.isSearching = false
		p.livePrefix = oldLivePrefix
		p.enableLivePrefix = oldEnableLivePrefix
		buf.InsertText(selected, false, true)
		if buf.Text() == lastSearchKey+searchSuffix {
			// No suggestion choosed
			if keepOriginalInput {
				buf.DeleteBeforeCursor(len(searchSuffix))
			} else {
				buf.DeleteBeforeCursor(len(buf.Text()))
			}
		} else {
			prompt.GoLineBeginning(buf)
			buf.Delete(len(lastSearchKey) + len(searchSuffix))
			prompt.GoLineEnd(buf)
		}
		return buf.Text(), len(searchSuffix) + len(selected) + len(searchKey)
	}
}

func (p *promptState) navigateHistory(origInput string, older bool, buf *prompt.Buffer) {
	if p.isSearching {
		return
	}
	if older {
		if p.historyNavPos < len(p.history) {
			p.historyNavPos++
		}

	} else {
		if p.historyNavPos > 0 {
			p.historyNavPos--
		}
	}
	prompt.GoLineBeginning(buf)
	buf.Delete(len([]rune(buf.Text())))
	if p.historyNavPos == 0 {
		buf.InsertText(origInput, false, true)
	} else {
		buf.InsertText(p.history[p.historyNavPos-1].Text, false, true)
	}
}

func (p *promptState) updateHistory() {
	p.historyNavPos = 0
	input := strings.Join(p.statements, " ")
	lastInput := strings.Join(p.lastStatements, " ")
	if len(p.statements) != 0 && input != lastInput {
		f, err := os.OpenFile(p.historyFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		w := bufio.NewWriter(f)
		stmt := strings.ReplaceAll(input, "\n", " ")
		fmt.Fprintf(w, "%s\n", stmt)
		w.Flush()
		p.history = append([]prompt.Suggest{prompt.Suggest{Text: stmt}}, p.history...)
	}
}

// lookaheadKeyword return the last keyword, the word before the last keyword, and the last word in words
func (p *promptState) lookaheadKeyword(words []string) (string, string, string) {
	if l := len(words); l > 0 {
		for i := l; i > 0; i-- {
			for _, k := range p.keywords {
				if strings.ToUpper(words[i-1]) == k {
					if i > 2 {
						return k, words[i-2], words[l-1]
					}
					return k, "", words[l-1]
				}
			}
		}
		return "", "", words[l-1]
	}
	return "", "", ""
}

func (p *promptState) clauseUnderCursor(in prompt.Document) (string, string, string) {
	// TODO(shendiaomo): use SQLFlow lexer to replace strings.Fields
	words := strings.Fields(strings.Join(p.statements, " ") + " " + in.TextBeforeCursor())
	return p.lookaheadKeyword(words)
}

func getOptimizerSuggestion(estimatorCls string, optimizers map[string]string) (r []prompt.Suggest) {
	if strings.HasPrefix(estimatorCls, "xgboost") || strings.HasPrefix(estimatorCls, "BoostedTrees") {
		return
	}
	if len(optimizers) == 0 {
		// Specify default optimizers
		// TODO(shendiaomo): Try to get the default value from the python `inspect` module instead of hard coding
		switch estimatorCls {
		case "LinearClassifier", "LinearRegressor":
			optimizers["optimizer"] = "Ftrl"
		case "DNNLinearCombinedClassifier", "DNNLinearCombinedRegressor":
			optimizers["dnn_optimizer"] = "Adagrad"
			optimizers["linear_optimizer"] = "Ftrl"
		default:
			optimizers["optimizer"] = "Adagrad"
		}
	}
	for key, opt := range optimizers {
		if params, ok := attribute.OptimizerParamsDocs[opt]; ok {
			// Construct suggestions for the specified optimizer
			r = append(r, prompt.Suggest{key, ""})
			for param, doc := range params {
				r = append(r, prompt.Suggest{key + "." + param, doc})
			}
		}
	}
	sortPromptSuggest(r)
	return
}

func (p *promptState) completer(in prompt.Document) []prompt.Suggest {
	if p.isSearching {
		return p.searchHistory()
	}
	w1 := in.GetWordBeforeCursor() // empty if the cursor is under whitespace
	clause, w0, w2 := p.clauseUnderCursor(in)
	switch clause {
	case "TO":
		return prompt.FilterHasPrefix(toSuggestions, w1, true)
	case "TRAIN":
		if w1 == "" {
			if clause == strings.ToUpper(w2) {
				return prompt.FilterHasPrefix(p.models, w1, true)
			}
			return prompt.FilterHasPrefix(trainSuggestions, w1, true)
		}
		return prompt.FilterHasPrefix(append(p.models, trainSuggestions...), w1, true)
	case "WITH":
		attributes := p.modelParamDocs[w0]
		if w0 != p.estimatorCls {
			p.estimatorCls = w0
			p.optimizers = make(map[string]string)
		}
		if len(attributes) != 0 {
			attributes = append(attributes, getOptimizerSuggestion(w0, p.optimizers)...)
		}
		if w1 == "" {
			if clause == strings.ToUpper(w2) || strings.HasSuffix(w2, ",") {
				return prompt.FilterHasPrefix(attributes, w1, true)
			}
			return prompt.FilterHasPrefix(withSuggestions, w1, true)
		}
		// The attribute is under editing
		attr := strings.Split(w1, "=")
		if len(attr) == 2 { // FIXME(shendiaomo): copy-n-paste doesn't work here
			switch attr[0] {
			case "model.optimizer", "model.dnn_optimizer", "model.linear_optimizer":
				if strings.HasSuffix(attr[1], ",") {
					paramName := strings.Split(attr[0], ".")[1]
					p.optimizers[paramName] = attr[1][0 : len(attr[1])-1] // cache the optimizer specified
				}
				var optimizerSuggest []prompt.Suggest
				for opt := range attribute.OptimizerParamsDocs {
					optimizerSuggest = append(optimizerSuggest, prompt.Suggest{opt, ""})
				}
				sortPromptSuggest(optimizerSuggest)
				return prompt.FilterHasPrefix(optimizerSuggest, attr[1], true)
			case "objective":
				if strings.HasPrefix(p.estimatorCls, "xgboost.") {
					var objectiveSuggest []prompt.Suggest
					for opt := range attribute.XGBoostObjectiveDocs {
						objectiveSuggest = append(objectiveSuggest, prompt.Suggest{opt, ""})
					}
					sortPromptSuggest(objectiveSuggest)
					return prompt.FilterHasPrefix(objectiveSuggest, attr[1], true)
				}
			}
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
		prefix:     prefix,
		livePrefix: livePrefix,
	}
	s.initCompleter()
	s.initHistory()
	return &s
}

var consoleWriter = newStdoutWriter()
var consoleParser *stdinParser

func getTerminalColumnSize() int {
	if consoleParser != nil {
		return int(consoleParser.GetWinSize().Col)
	}
	return 1024 // This is for passing travis unit test
}

func runPrompt(cb func(string)) {
	state := newPromptState()
	consoleParser = newStdinParser()
	p := prompt.New(
		func(in string) { state.execute(in, cb) },
		func(in prompt.Document) []prompt.Suggest { return state.completer(in) },
		prompt.OptionAddASCIICodeBind(emacsMetaKeyBindings...),
		prompt.OptionAddKeyBind(emacsCtrlKeyBindings...),
		prompt.OptionLivePrefix(func() (string, bool) { return state.changeLivePrefix() }),
		prompt.OptionSwitchKeyBindMode(prompt.CommonKeyBind),
		prompt.OptionWriter(consoleWriter),
		prompt.OptionParser(consoleParser),
		prompt.OptionPrefix(state.prefix),
		prompt.OptionPrefixTextColor(prompt.DefaultColor),
		prompt.OptionTitle("SQLFlow"),
		prompt.OptionCompletionWordSeparator(" ="),
		prompt.OptionMaxSuggestion(10),
	)
	fmt.Println("Welcome to SQLFlow.  Commands end with ;")
	fmt.Println()
	p.Run()
	fmt.Println("Goodbye!")
}
