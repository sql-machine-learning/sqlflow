package sql

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	_, ch := newLexer("+-***/()<<==", lexOperator)

	typs := []itemType{
		itemPlus, itemMinus, itemPower, itemTimes, itemDivides,
		itemLeftParen, itemRightParen, itemLess, itemLessEqual,
		itemEqual}

	vals := []string{
		"+", "-", "**", "*", "/", "(", ")", "<", "<=", "="}

	i := 0
	for v := range ch {
		assert.Equal(t, typs[i], v.typ)
		assert.Equal(t, vals[i], v.val)
		i++
	}
}

func TestLexIdentOrKeyword(t *testing.T) {
	_, ch := newLexer("a12b", lexIdentOrKeyword)
	v := <-ch
	assert.Equal(t, itemIdent, v.typ)
	assert.Equal(t, "a12b", v.val)

	_, ch = newLexer("Select", lexIdentOrKeyword)
	v = <-ch
	assert.Equal(t, itemSelect, v.typ)
	assert.Equal(t, "Select", v.val)

	_, ch = newLexer("froM", lexIdentOrKeyword)
	v = <-ch
	assert.Equal(t, itemFrom, v.typ)
	assert.Equal(t, "froM", v.val)
}

func TestLexToken(t *testing.T) {
	_, ch := newLexer("  Select * from a_table where col > 100;", lexToken)
	for v := range ch {
		fmt.Println(v)
	}
}
