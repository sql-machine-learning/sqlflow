package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLexer(t *testing.T) {
	_, ch := newLexer(" 123;", func(l *lexer) lexState { return nil })
	n := 0
	for range ch {
		n++
	}
	assert.Equal(t, n, 0)
}

func TestCanStop(t *testing.T) {
	_, ch := newLexer(";", lexOperator)
	for range ch {
	}
}

func TestItemError(t *testing.T) {
	_, ch := newLexer("  ;", lexOperator) // lexOperator expects no space
	v := <-ch
	assert.Equal(t, v.typ, itemError)
	for range ch {
	}
}

func TestLexNumber(t *testing.T) {
	_, ch := newLexer("123", lexNumber)
	v := <-ch
	assert.Equal(t, itemNumber, v.typ)
	assert.Equal(t, "123", v.val)

	_, ch = newLexer("abc", lexNumber)
	v = <-ch
	assert.Equal(t, itemError, v.typ)
}

func TestLexOperator(t *testing.T) {
	_, ch := newLexer("+-***/()", lexOperator)
	v := <-ch
	assert.Equal(t, itemPlus, v.typ)
	assert.Equal(t, v.val, "+")

	v = <-ch
	assert.Equal(t, itemMinus, v.typ)
	assert.Equal(t, "-", v.val)

	v = <-ch
	assert.Equal(t, itemPower, v.typ)
	assert.Equal(t, "**", v.val)

	v = <-ch
	assert.Equal(t, itemTimes, v.typ)
	assert.Equal(t, "*", v.val)

	v = <-ch
	assert.Equal(t, itemDivides, v.typ)
	assert.Equal(t, "/", v.val)

	v = <-ch
	assert.Equal(t, itemLeftParen, v.typ)
	assert.Equal(t, "(", v.val)

	v = <-ch
	assert.Equal(t, itemRightParen, v.typ)
	assert.Equal(t, ")", v.val)
}
