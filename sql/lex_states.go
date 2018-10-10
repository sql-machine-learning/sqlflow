package sql

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	itemError itemType = iota

	itemIdent
	itemSelect
	itemFrom
	itemWhere
	itemLimit

	itemNumber

	itemPlus
	itemMinus
	itemTimes
	itemDivides
	itemPower
	itemLeftParen
	itemRightParen
	itemGreater
	itemLess
	itemGreaterEqual
	itemLessEqual
	itemEqual
	itemComma

	itemSemiColon
)

func lexToken(l *lexer) lexState {
	l.skipSpaces()
	r := l.peek()
	switch {
	case unicode.IsLetter(r):
		return lexIdentOrKeyword(l)
	case unicode.IsDigit(r):
		return lexNumber(l)
	case r == '-' || r == '+':
		// A hacky further look ahead
		rr, _ := utf8.DecodeRuneInString(l.input[l.pos+1:])
		if unicode.IsDigit(rr) {
			return lexNumber(l)
		} else {
			return lexOperator(l)
		}
	case strings.IndexRune("*/<>=(),;", r) >= 0:
		return lexOperator(l)
	}
	return nil // including the case of eof.
}

func lexIdentOrKeyword(l *lexer) lexState {
	// lexToken ensured that the first rune is a letter.
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

	switch strings.ToUpper(l.input[l.start:l.pos]) {
	case "SELECT":
		l.emit(itemSelect)
	case "FROM":
		l.emit(itemFrom)
	case "WHERE":
		l.emit(itemWhere)
	case "LIMIT":
		l.emit(itemLimit)
	default:
		l.emit(itemIdent)
	}
	return lexToken
}

var (
	// Stolen https://www.regular-expressions.info/floatingpoint.html.
	reNumber = regexp.MustCompile("[-+]?[0-9]*.?[0-9]+([eE][-+]?[0-9]+)?")
)

func lexNumber(l *lexer) lexState {
	m := reNumber.FindStringIndex(l.input[l.pos:])
	if m == nil || m[0] != 0 {
		l.emitError("Expecting a number, but see %.10q", l.input[l.pos:])
		return nil // stop lexing.
	}
	l.pos += m[1]
	l.emit(itemNumber)
	return lexToken
}

func lexOperator(l *lexer) lexState {
	switch r := l.next(); r {
	case '+':
		l.emit(itemPlus)
	case '-':
		l.emit(itemMinus)
	case '*':
		if l.peek() == '*' {
			l.next()
			l.emit(itemPower)
		} else {
			l.emit(itemTimes)
		}
	case '/':
		l.emit(itemDivides)
	case '(':
		l.emit(itemLeftParen)
	case ')':
		l.emit(itemRightParen)
	case '=':
		l.emit(itemEqual)
	case '<':
		if l.peek() == '=' {
			l.next()
			l.emit(itemLessEqual)
		} else {
			l.emit(itemLess)
		}
	case '>':
		if l.peek() == '=' {
			l.next()
			l.emit(itemGreaterEqual)
		} else {
			l.emit(itemGreater)
		}
	case ',':
		l.emit(itemComma)
	case ';':
		l.emit(itemSemiColon)
		return nil // ; marks the end of a statement
	default:
		l.emitError("lexOperator: unknown character %c", r)
		return nil
	}
	return lexToken
}
