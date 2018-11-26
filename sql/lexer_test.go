package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLexer(t *testing.T) {
	a := assert.New(t)
	l := newLexer("")
	var n sqlSymType
	a.Equal(0, l.Lex(&n))
}

func TestNextAndBackup(t *testing.T) {
	a := assert.New(t)
	l := newLexer("ab")
	a.Equal('a', l.next())
	l.backup()
	a.Equal('a', l.next())
	a.Equal('b', l.next())
	a.Equal(eof, l.next())
	a.Equal(eof, l.next())
	l.backup()
	a.Equal(eof, l.next())
}

func TestSkipSpaces(t *testing.T) {
	a := assert.New(t)
	l := newLexer("ab")
	l.skipSpaces()
	a.Equal('a', rune(l.input[l.start]))
	a.Equal('a', l.next())
	l.skipSpaces()
	a.Equal('b', rune(l.input[l.start]))
	a.Equal('b', l.next())
}

func TestLexNumber(t *testing.T) {
	a := assert.New(t)
	l := newLexer("123.4")
	var n sqlSymType
	a.Equal(NUMBER, l.Lex(&n))
	a.Equal("123.4", n.val)
}

func TestLexString(t *testing.T) {
	a := assert.New(t)
	l := newLexer(`  "\""  `)
	var n sqlSymType
	a.Equal(STRING, l.Lex(&n))
	a.Equal(`"\""`, n.val)
}

func TestLexOperator(t *testing.T) {
	a := assert.New(t)
	l := newLexer(`+-***/%()[]{}<<==,;`)

	typs := []int{
		'+', '-', POWER, '*', '/', '%', '(', ')', '[', ']', '{', '}',
		'<', LE, '=', ',', ';'}
	vals := []string{
		"+", "-", "**", "*", "/", "%", "(", ")", "[", "]",
		"{", "}", "<", "<=", "=", ",", ";"}
	i := 0
	var n sqlSymType
	for typ := l.Lex(&n); typ != 0; typ = l.Lex(&n) {
		a.Equal(typs[i], typ)
		a.Equal(vals[i], n.val)
		i++
	}
}

func TestLexIdentOrKeyword(t *testing.T) {
	a := assert.New(t)
	vals := []string{"a1_2b", "x.y", "x.y.z", "Select", "froM", "where", "tRain", "colUmn",
		"and", "or", "not"}
	typs := []int{IDENT, IDENT, IDENT, SELECT, FROM, WHERE, TRAIN, COLUMN,
		AND, OR, NOT}
	var n sqlSymType
	for i, it := range vals {
		l := newLexer(it)
		a.Equal(typs[i], l.Lex(&n))
		a.Equal(vals[i], n.val)
	}
}

func TestLexSQL(t *testing.T) {
	a := assert.New(t)
	l := newLexer("  Select * from a_table where a_table.col_1 > 100;")
	typs := []int{
		SELECT, '*', FROM, IDENT, WHERE, IDENT, '>', NUMBER, ';'}
	vals := []string{
		"Select", "*", "from", "a_table", "where",
		"a_table.col_1", ">", "100", ";"}
	var n sqlSymType
	for i := range typs {
		a.Equal(typs[i], l.Lex(&n))
		a.Equal(vals[i], n.val)
	}
}
