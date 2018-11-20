package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLexer(t *testing.T) {
	l := newLexer("")
	var n sqlSymType
	assert.Equal(t, 0, l.Lex(&n))
}

func TestNextAndBackup(t *testing.T) {
	l := newLexer("ab")
	assert.Equal(t, 'a', l.next())
	l.backup()
	assert.Equal(t, 'a', l.next())
	assert.Equal(t, 'b', l.next())
	assert.Equal(t, eof, l.next())
	assert.Equal(t, eof, l.next())
	l.backup()
	assert.Equal(t, eof, l.next())
}

func TestSkipSpaces(t *testing.T) {
	l := newLexer("ab")
	l.skipSpaces()
	assert.Equal(t, 'a', rune(l.input[l.start]))
	assert.Equal(t, 'a', l.next())
	l.skipSpaces()
	assert.Equal(t, 'b', rune(l.input[l.start]))
	assert.Equal(t, 'b', l.next())
}

func TestLexNumber(t *testing.T) {
	l := newLexer("123.4")
	var n sqlSymType

	assert.Equal(t, NUMBER, l.Lex(&n))
	assert.Equal(t, "123.4", n.val)
}

func TestLexString(t *testing.T) {
	l := newLexer(`  "\""  `)
	var n sqlSymType
	assert.Equal(t, STRING, l.Lex(&n))
	assert.Equal(t, `"\""`, n.val)
}

func TestLexOperator(t *testing.T) {
	l := newLexer("+-***/%()[]{}<<==,;")

	typs := []int{
		'+', '-', POWER, '*', '/', '%', '(', ')', '[', ']', '{', '}',
		'<', LE, '=', ',', ';'}
	vals := []string{
		"+", "-", "**", "*", "/", "%", "(", ")", "[", "]",
		"{", "}", "<", "<=", "=", ",", ";"}
	i := 0
	var n sqlSymType
	for typ := l.Lex(&n); typ != 0; typ = l.Lex(&n) {
		assert.Equal(t, typs[i], typ)
		assert.Equal(t, vals[i], n.val)
		i++
	}
}

func TestLexIdentOrKeyword(t *testing.T) {
	vals := []string{"a1_2b", "x.y", "x.y.z", "Select", "froM", "where", "tRain", "colUmn",
		"and", "or", "not"}
	typs := []int{IDENT, IDENT, IDENT, SELECT, FROM, WHERE, TRAIN, COLUMN,
		AND, OR, NOT}
	var n sqlSymType
	for i, it := range vals {
		l := newLexer(it)
		assert.Equal(t, typs[i], l.Lex(&n))
		assert.Equal(t, vals[i], n.val)
	}
}

func TestLexSQL(t *testing.T) {
	l := newLexer("  Select * from a_table where a_table.col_1 > 100;")
	typs := []int{
		SELECT, '*', FROM, IDENT, WHERE, IDENT, '>', NUMBER, ';'}
	vals := []string{
		"Select", "*", "from", "a_table", "where",
		"a_table.col_1", ">", "100", ";"}
	var n sqlSymType
	for i := range typs {
		assert.Equal(t, typs[i], l.Lex(&n))
		assert.Equal(t, vals[i], n.val)
	}
}
