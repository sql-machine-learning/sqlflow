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

	itemNumber

	itemPlus
	itemMinus
	itemTimes
	itemDivides
	itemPower
	itemLeftParen
	itemRightParen
	itemSemiColon
)

func lexToken(l *lexer) lexState {
	l.acceptSpaces()
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
	case unicode.IsPunct(r):
		return lexOperator(l)
	}
	return nil // including the case of eof.
}

func lexIdentOrKeyword(l *lexer) lexState {
	r := l.next()
	for unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
	}

	switch strings.ToUpper(l.input[l.start:l.pos]) {
	case "SELECT":
		l.emit(itemSelect)
	case "FROM":
		l.emit(itemFrom)
	default:
		l.emit(itemIdent)
	}
	return lexToken
}

var (
	regexpNumber = regexp.MustCompile("[-+]?[0-9]*.?[0-9]+([eE][-+]?[0-9]+)?")
)

func lexNumber(l *lexer) lexState {
	m := regexpNumber.FindStringIndex(l.input[l.pos:])
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
	case ';':
		l.emit(itemSemiColon)
		return nil // ; marks the end of a statement
	default:
		l.emitError("lexOperator: unknown character %c", r)
		return nil
	}
	return lexToken
}
