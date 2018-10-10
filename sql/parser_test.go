package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSelect(t *testing.T) {
	_, ch := newLexer("SELECT image, label FROM mnist_train;", lexToken)
	p := newParser(ch)
	p.parse()
	assert.Equal(t, []string{"image", "label"}, p.sel.fields)
	assert.Equal(t, []string{"mnist_train"}, p.sel.from)
}
