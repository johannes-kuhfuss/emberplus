package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewEmberClient_Returns_EmberClient(t *testing.T) {
	ec := NewEmberClient("localhost", 9000)
	assert.NotNil(t, ec)
	assert.EqualValues(t, "localhost:9000", ec.Addr)
}
