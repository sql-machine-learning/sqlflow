package gohive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDSN(t *testing.T) {
	cfg, e := parseDSN("root:root@127.0.0.1/mnist")
	assert.Nil(t, e)
	assert.Equal(t, cfg.User, "root")
	assert.Equal(t, cfg.Passwd, "root")
	assert.Equal(t, cfg.Addr, "127.0.0.1")
	assert.Equal(t, cfg.DBName, "mnist")

	cfg, e = parseDSN("root@127.0.0.1/mnist")
	assert.Nil(t, e)
	assert.Equal(t, cfg.User, "root")
	assert.Equal(t, cfg.Passwd, "")
	assert.Equal(t, cfg.Addr, "127.0.0.1")
	assert.Equal(t, cfg.DBName, "mnist")

	cfg, e = parseDSN("127.0.0.1/mnist")
	assert.Nil(t, e)
	assert.Equal(t, cfg.User, "")
	assert.Equal(t, cfg.Passwd, "")
	assert.Equal(t, cfg.Addr, "127.0.0.1")
	assert.Equal(t, cfg.DBName, "mnist")

	cfg, e = parseDSN("127.0.0.1")
	assert.NotNil(t, e) // Need database name.
}
