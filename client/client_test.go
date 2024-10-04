package client

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewEmberClient_WrongPort_Returns_Error(t *testing.T) {
	ec, err := NewEmberClient("localhost", -1)
	assert.Nil(t, ec)
	assert.NotNil(t, err)
	assert.EqualValues(t, "port must be between 1 and 65535", err.Error())
}

func Test_NewEmberClient_NoHost_Returns_Error(t *testing.T) {
	ec, err := NewEmberClient("", 9000)
	assert.Nil(t, ec)
	assert.NotNil(t, err)
	assert.EqualValues(t, "host must be either a host name or an IP address", err.Error())
}

func Test_NewEmberClient_HostName_Returns_EmberClient(t *testing.T) {
	ec, err := NewEmberClient("localhost", 9000)
	assert.NotNil(t, ec)
	assert.Nil(t, err)
	assert.EqualValues(t, "localhost:9000", ec.raddr)
}

func Test_NewEmberClient_IP_Returns_EmberClient(t *testing.T) {
	ec, err := NewEmberClient("127.0.0.1", 9000)
	assert.NotNil(t, ec)
	assert.Nil(t, err)
	assert.EqualValues(t, "127.0.0.1:9000", ec.raddr)
}

func Test_Connect_CannotConnect_Returns_Error(t *testing.T) {
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	err := ec.Connect()
	assert.NotNil(t, err)
	assert.EqualValues(t, false, ec.IsConnected())
	assert.EqualValues(t, "dial tcp 127.0.0.1:9000: connectex: No connection could be made because the target machine actively refused it.", err.Error())
}

func Test_Connect_CanConnect_Returns_NoError(t *testing.T) {
	c, _ := net.Listen("tcp", "127.0.0.1:9000")
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	err := ec.Connect()
	assert.Nil(t, err)
	assert.EqualValues(t, true, ec.IsConnected())
	ec.Disconnect()
	c.Close()
}

func Test_Connect_AlreadyConnected_Returns_Error(t *testing.T) {
	c, _ := net.Listen("tcp", "127.0.0.1:9000")
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	ec.Connect()
	err := ec.Connect()
	assert.NotNil(t, err)
	assert.EqualValues(t, "already connected", err.Error())
	ec.Disconnect()
	c.Close()
}

func Test_Disconnect_NoConnection_Returns_Error(t *testing.T) {
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	err := ec.Disconnect()
	assert.NotNil(t, err)
	assert.EqualValues(t, "not connected", err.Error())
}

func Test_Disconnect_Connected_Returns_NoError(t *testing.T) {
	c, _ := net.Listen("tcp", "127.0.0.1:9000")
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	ec.Connect()
	err := ec.Disconnect()
	assert.Nil(t, err)
	c.Close()
}
