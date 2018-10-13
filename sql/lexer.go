// The lexer in this package is a copy of https://talks.golang.org/2011/lex.slide#1.
package sql

import (
	"log"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	eof rune = 0 // a special rune
)

type item struct {
	typ int
	sym sqlSymType // sqlSymType is defined in sql.y
}

type lexer struct {
	input string // the string being scanned
	start int    // start position of this item
	pos   int    // current position in the input
	width int    // width of last rune read from input
}

func newLexer(input string) *lexer {
	return &lexer{input: input}
}

func (l *lexer) Error(e string) {
	log.Panicf("start=%d, pos=%d : %s\n%.10q\n", l.start, l.pos, e, l.input[l.start:])
}

func (l *lexer) emit(lval *sqlSymType, typ int) int {
	lval.val = l.input[l.start:l.pos]
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

func (l *lexer) ignore() {
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

func (l *lexer) skipSpaces() {
	for r := l.next(); unicode.IsSpace(r); r = l.next() {
	}
	l.backup()
	l.start = l.pos
}

func (l *lexer) Lex(lval *sqlSymType) int {
	l.skipSpaces()
	r := l.peek()
	switch {
	case unicode.IsLetter(r):
		return l.lexIdentOrKeyword(lval)
	case unicode.IsDigit(r):
		return l.lexNumber(lval)
	case strings.IndexRune("+-*/%<>=()[]{},;", r) >= 0:
		return l.lexOperator(lval)
	case r == eof:
		return 0 // indicate the end of lexing.
	}
	log.Panicf("Lex: Unknown problem %s", l.input[l.start:])
	return -1 // indicate an error
}

func (l *lexer) lexIdentOrKeyword(lval *sqlSymType) int {
	// lexToken ensures that the first rune is a letter.
	r := l.next()
	for unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
		r = l.next()
	}
	if r == '.' { // SQL identification can contain a dot.
		r = l.next()
		for unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
			r = l.next()
		}
	}
	l.backup()

	return l.emitIdentOrKeyword(lval)
}

func (l *lexer) emitIdentOrKeyword(lval *sqlSymType) int {
	keywds := map[string]int{
		"SELECT": SELECT,
		"FROM":   FROM,
		"WHERE":  WHERE,
		"LIMIT":  LIMIT,
		"TRAIN":  TRAIN,
		"COLUMN": COLUMN,
	}
	if typ, ok := keywds[strings.ToUpper(l.input[l.start:l.pos])]; ok {
		return l.emit(lval, typ)
	}
	return l.emit(lval, IDENT)
}

var (
	// Stolen https://www.regular-expressions.info/floatingpoint.html.
	reNumber = regexp.MustCompile("[-+]?[0-9]*.?[0-9]+([eE][-+]?[0-9]+)?")
)

func (l *lexer) lexNumber(lval *sqlSymType) int {
	m := reNumber.FindStringIndex(l.input[l.pos:])
	if m == nil || m[0] != 0 {
		log.Panicf("Expecting a number, but see %.10q", l.input[l.pos:])
	}
	l.pos += m[1]
	return l.emit(lval, NUMBER)
}

func (l *lexer) lexOperator(lval *sqlSymType) int {
	r := l.next()
	switch r {
	case '*':
		if l.peek() == '*' {
			l.next()
			return l.emit(lval, POWER)
		}
	case '<':
		if l.peek() == '=' {
			l.next()
			return l.emit(lval, LE)
		}
	case '>':
		if l.peek() == '=' {
			l.next()
			return l.emit(lval, GE)
		}
	}
	return l.emit(lval, int(r))
}
