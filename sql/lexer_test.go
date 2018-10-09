package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLexer(t *testing.T) {
	_, ch := newLexer("", func(l *lexer) lexState { return nil })
	n := 0
	for range ch {
		n++
	}
	assert.Equal(t, n, 0)
}

func TestNextAndBackup(t *testing.T) {
	l := lexer{input: "ab"}
	assert.Equal(t, 'a', l.next())
	l.backup()
	assert.Equal(t, 'a', l.next())
	assert.Equal(t, 'b', l.next())
	assert.Equal(t, eof, l.next())
	assert.Equal(t, eof, l.next())
	l.backup()
	assert.Equal(t, eof, l.next())
}

func TestAccept(t *testing.T) {
	l := lexer{input: "abc"}
	l.accept(func(rune) bool { return false })
	assert.Equal(t, 'a', l.next())
	l.accept(func(rune) bool { return true })
	assert.Equal(t, 'c', l.next())
}

func TestAcceptOne(t *testing.T) {
	l := lexer{input: "abc"}
	l.acceptOne("abc")
	assert.Equal(t, 'b', l.next())
	l.acceptOne("")
	assert.Equal(t, 'c', l.next())
}

func TestAcceptRun(t *testing.T) {
	l := lexer{input: " \t ab"}
	l.acceptRun(" \t")
	assert.Equal(t, 'a', l.next())
	l.acceptRun(" ")
	assert.Equal(t, 'b', l.next())
}

func TestAcceptSpaces(t *testing.T) {
	l := lexer{input: " \t ab"}
	l.acceptSpaces()
	assert.Equal(t, 'a', l.next())
	l.acceptSpaces()
	assert.Equal(t, 'b', l.next())
}
