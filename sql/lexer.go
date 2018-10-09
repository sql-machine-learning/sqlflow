// The lexer in this package is a copy of https://talks.golang.org/2011/lex.slide#1.
package sql

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	eof rune = 0 // a special rune
)

type itemType int

type item struct {
	typ itemType
	val string
}

type lexer struct {
	input string    // the string being scanned
	start int       // start position of this item
	pos   int       // current position in the input
	width int       // width of last rune read from input
	items chan item // channel of scanned items
}

type lexState func(*lexer) lexState

func newLexer(input string, initial lexState) (*lexer, chan item) {
	l := &lexer{
		input: input,
		start: 0,
		pos:   0,
		width: 0,
		items: make(chan item),
	}
	go l.run(initial)
	return l, l.items
}

func (l *lexer) run(initial lexState) {
	for state := initial; state != nil; {
		state = state(l)
	}
	close(l.items)
}

func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.input[l.start:l.pos]}
	l.start = l.pos
}

func (l *lexer) emitError(format string, a ...interface{}) {
	l.items <- item{itemError, fmt.Sprintf(format, a...)}
	l.start = l.pos
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

func (l *lexer) ignoe() {
	l.start = l.pos
}

func (l *lexer) backup() {
	l.pos -= l.width // when next() return eof, l.width==0.
}

func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

func (l *lexer) accept(predicate func(rune) bool) bool {
	r := predicate(l.next())
	l.backup()
	return r
}

func (l *lexer) acceptOne(valid string) bool {
	return l.accept(func(r rune) bool {
		return strings.IndexRune(valid, r) >= 0
	})
}

func (l *lexer) acceptRun(valid string) {
	for l.acceptOne(valid) {
	}
}

func (l *lexer) acceptSpaces() {
	for l.accept(unicode.IsSpace) {
	}
}
