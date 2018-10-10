package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSelect(t *testing.T) {
	_, ch := newLexer("SELECT image, label FROM mnist_train LIMIT 321 TRAIN DNNClassifier;", lexToken)
	p := newParser(ch)
	p.parse()
	assert.Equal(t, []string{"image", "label"}, p.sel.fields)
	assert.Equal(t, []string{"mnist_train"}, p.sel.from)
	assert.Equal(t, 321, p.sel.limit)
	assert.Equal(t, "DNNClassifier", p.sel.estimator)
}
