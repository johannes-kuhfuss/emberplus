package emberclient

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

/* Example of how to call client

ec, _ := client.NewEmberClient("192.168.200.55", 9000)
ec.Connect()
defer ec.Disconnect()

data, err := ec.GetRoot()
if err != nil {
	logger.Error("oops", err)
} else {
	logger.Info(fmt.Sprintf("Data returned: %v", string(data)))
}
*/

func TestNewEmberClientWrongPortReturnsError(t *testing.T) {
	ec, err := NewEmberClient("localhost", -1)
	assert.Nil(t, ec)
	assert.NotNil(t, err)
	assert.EqualValues(t, "port must be between 1 and 65535", err.Error())
}

func TestNewEmberClientNoHostReturnsError(t *testing.T) {
	ec, err := NewEmberClient("", 9000)
	assert.Nil(t, ec)
	assert.NotNil(t, err)
	assert.EqualValues(t, "host must be either a host name or an IP address", err.Error())
}

func TestNewEmberClientHostNameReturnsEmberClient(t *testing.T) {
	ec, err := NewEmberClient("localhost", 9000)
	assert.NotNil(t, ec)
	assert.Nil(t, err)
	assert.EqualValues(t, "localhost:9000", ec.raddr)
}

func TestNewEmberClientIPReturnsEmberClient(t *testing.T) {
	ec, err := NewEmberClient("127.0.0.1", 9000)
	assert.NotNil(t, ec)
	assert.Nil(t, err)
	assert.EqualValues(t, "127.0.0.1:9000", ec.raddr)
}

func TestConnectCannotConnectReturnsError(t *testing.T) {
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	err := ec.Connect()
	assert.NotNil(t, err)
	assert.EqualValues(t, false, ec.IsConnected())
	assert.EqualValues(t, "dial tcp 127.0.0.1:9000: connectex: No connection could be made because the target machine actively refused it.", err.Error())
}

func TestConnectCanConnectReturnsNoError(t *testing.T) {
	c, _ := net.Listen("tcp", "127.0.0.1:9000")
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	err := ec.Connect()
	assert.Nil(t, err)
	assert.EqualValues(t, true, ec.IsConnected())
	ec.Disconnect()
	c.Close()
}

func TestConnectAlreadyConnectedReturnsError(t *testing.T) {
	c, _ := net.Listen("tcp", "127.0.0.1:9000")
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	ec.Connect()
	err := ec.Connect()
	assert.NotNil(t, err)
	assert.EqualValues(t, "already connected", err.Error())
	ec.Disconnect()
	c.Close()
}

func TestDisconnectNoConnectionReturnsError(t *testing.T) {
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	err := ec.Disconnect()
	assert.NotNil(t, err)
	assert.EqualValues(t, "not connected", err.Error())
}

func TestDisconnectConnectedReturnsNoError(t *testing.T) {
	c, _ := net.Listen("tcp", "127.0.0.1:9000")
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	ec.Connect()
	err := ec.Disconnect()
	assert.Nil(t, err)
	c.Close()
}
