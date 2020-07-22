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

package parser

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	eof rune = 0 // a special rune
)

// The lexer in this package is inspired by Rob Pike's 2011 talk on
// writing lexers manually https://talks.golang.org/2011/lex.slide#1.
// It makes a significant simplification of the idea and doesn't use
// goroutines and channels.
type lexer struct {
	input    string // the string being scanned
	start    int    // start position of this item
	pos      int    // current position in the input
	width    int    // width of last rune read from input
	err      error  // the parser could return the error
	previous int    // previous start, recorded for error position
	// parse result
	result *SQLFlowSelectStmt
}

func newLexer(input string) *lexer {
	return &lexer{input: input}
}

// Error records e in lexer.err so that parseSQLFlowStmt could return.
func (l *lexer) Error(e string) {
	l.err = fmt.Errorf(
		"%v near or before %.30q. %s",
		e, l.input[l.previous:],
		"You might want to refer to the https://sqlflow.org/sqlflow/doc/language_guide")
}

func (l *lexer) emit(lval *extendedSyntaxSymType, typ int) int {
	lval.val = l.input[l.start:l.pos]
	l.previous = l.start
	l.start = l.pos
	return typ
}

func (l *lexer) next() (r rune) {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return r
}

func (l *lexer) backup() {
	l.pos -= l.width // when next() return eof, l.width==0.
}

func (l *lexer) peek() rune {
	if l.pos >= len(l.input) {
		// do not call l.next() in this case, because it cannot
		// backup when eof.
		return eof
	}
	r := l.next()
	l.backup()
	return r
}

func (l *lexer) skipSpaces() int {
	skipNum := 0
	for r := l.next(); unicode.IsSpace(r); r = l.next() {
		skipNum++
	}
	l.backup()
	l.start = l.pos
	return skipNum
}

func (l *lexer) skipSingleLineComment() int {
	nextRune := l.peek()
	if nextRune != '-' {
		return 0
	}

	l.next()
	nextNextRune := l.peek()
	if nextNextRune != '-' {
		l.backup()
		return 0
	}
	l.next()

	skipNum := 0
	for r := l.next(); r != '\n' && r != eof; r = l.next() {
		skipNum++
	}
	l.backup()
	l.start = l.pos
	return skipNum
}

func (l *lexer) skipMultipleLineComment() int {
	nextRune := l.peek()
	if nextRune != '/' {
		return 0
	}

	l.next()
	nextNextRune := l.peek()
	if nextNextRune != '*' {
		l.backup()
		return 0
	}

	skipNum := 0
	for r := l.next(); r != '*' || l.peek() != '/'; r = l.next() {
		if r == eof {
			l.previous = l.start
			l.err = fmt.Errorf("cannot find the end (*/) of the comment /*...*/")
			return 0
		}
		skipNum++
	}
	l.next()
	l.start = l.pos
	return skipNum
}

func (l *lexer) Lex(lval *extendedSyntaxSymType) int {
	start, pos, width := l.start, l.pos, l.width
	hasSkipComment := false
	for {
		skipSpaceNum := l.skipSpaces()
		skipCommentNum := l.skipSingleLineComment() + l.skipMultipleLineComment()

		if skipCommentNum != 0 {
			hasSkipComment = true
		} else if skipSpaceNum == 0 { // skip 0 space and 0 comment
			break
		}
	}

	r := l.peek()
	switch {
	case unicode.IsLetter(r):
		return l.lexIdentOrKeyword(lval)
	case unicode.IsDigit(r):
		return l.lexNumber(lval)
	case r == '"' || r == '\'':
		return l.lexString(lval)
	case strings.IndexRune("+-*/%<>=()[]{},;!", r) >= 0:
		return l.lexOperator(lval)
	case r == eof:
		// If comment is the end of the statement, make the comment
		// be the start of the next statement. So that each statement
		// would be end with ";".
		if hasSkipComment {
			l.start, l.pos, l.width = start, pos, width
		}
		return 0 // indicate the end of lexing.
	}
	// return the position where the error was detected.
	return 0 - l.start
}

func (l *lexer) lexIdentOrKeyword(lval *extendedSyntaxSymType) int {
	// lexToken ensures that the first rune is a letter.
	r := l.next()
	for {
		// model IDENT may be like: some-domain.com/a_data_scientist/regressors:v0.2/MyDNNRegressor
		for unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '/' || r == ':' || r == '-' {
			r = l.next()
		}
		if r != '.' { // The dot cannot be the last rune.
			break
		} else {
			r = l.next()
		}
	}
	l.backup()

	return l.emitIdentOrKeyword(lval)
}

func (l *lexer) emitIdentOrKeyword(lval *extendedSyntaxSymType) int {
	keywds := map[string]int{
		"SELECT":     SELECT,
		"FROM":       FROM,
		"WHERE":      WHERE,
		"LIMIT":      LIMIT,
		"TRAIN":      TRAIN,
		"PREDICT":    PREDICT,
		"EXPLAIN":    EXPLAIN,
		"EVALUATE":   EVALUATE,
		"RUN":        RUN,
		"MAXIMIZE":   MAXIMIZE,
		"MINIMIZE":   MINIMIZE,
		"USING":      USING,
		"WITH":       WITH,
		"CMD":        CMD,
		"COLUMN":     COLUMN,
		"FOR":        FOR,
		"LABEL":      LABEL,
		"INTO":       INTO,
		"CONSTRAINT": CONSTRAINT,
		"AND":        AND,
		"OR":         OR,
		"NOT":        NOT,
		"AS":         AS,
		"TO":         TO,
		"SHOW":       SHOW,
		"GROUP":      GROUP,
		"BY":         BY,
	}
	if typ, ok := keywds[strings.ToUpper(l.input[l.start:l.pos])]; ok {
		return l.emit(lval, typ)
	}
	return l.emit(lval, IDENT)
}

var (
	reNumber = regexp.MustCompile("[-+]?[0-9]*[.]?[0-9]+([eE][-+]?[0-9]+)?")
)

func (l *lexer) lexNumber(lval *extendedSyntaxSymType) int {
	m := reNumber.FindStringIndex(l.input[l.pos:])
	if m == nil || m[0] != 0 {
		log.Panicf("Expecting a number, but see %.10q", l.input[l.pos:])
	}
	l.pos += m[1]
	return l.emit(lval, NUMBER)
}

func (l *lexer) lexOperator(lval *extendedSyntaxSymType) int {
	r := l.next()
	if r == '*' && l.peek() == '*' {
		l.next()
		return l.emit(lval, POWER)
	} else if r == '<' && l.peek() == '=' {
		l.next()
		return l.emit(lval, LE)
	} else if r == '>' && l.peek() == '=' {
		l.next()
		return l.emit(lval, GE)
	} else if r == '!' && l.peek() == '=' {
		l.next()
		return l.emit(lval, NE)
	} else if r == '<' && l.peek() == '>' {
		l.next()
		return l.emit(lval, NE)
	}
	return l.emit(lval, int(r))
}

func (l *lexer) lexString(lval *extendedSyntaxSymType) int {
	l.next() // the left quote
	for r := l.next(); r != '"' && r != '\''; r = l.next() {
		if r == eof {
			l.previous = l.start
			l.err = fmt.Errorf("unmatched quotation")
			return -l.start
		}
		if r == '\\' {
			l.next()
		}
	}
	return l.emit(lval, STRING)
}
