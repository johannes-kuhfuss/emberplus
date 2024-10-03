package client

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/johannes-kuhfuss/services_utils/logger"
)

type EmberClient struct {
	raddr string
	conn  net.Conn
}

func NewEmberClient(host string, port int) (*EmberClient, error) {
	var ec EmberClient
	if (port < 1) || (port > 65535) {
		return nil, errors.New("port must be between 1 and 65535")
	}
	if host == "" {
		return nil, errors.New("host must be either a host name or an IP address")
	}
	portStr := strconv.Itoa(port)
	ec.raddr = net.JoinHostPort(host, portStr)
	return &ec, nil
}

func (ec *EmberClient) IsConnected() bool {
	return ec.conn != nil
}

func (ec *EmberClient) Connect() error {
	if ec.IsConnected() {
		err := errors.New("already connected")
		logger.Error(fmt.Sprintf("Cannot connect to %v", ec.raddr), err)
		return err
	}
	logger.Info(fmt.Sprintf("Trying to connect to %v...", ec.raddr))
	conn, err := net.Dial("tcp", ec.raddr)
	if err != nil {
		logger.Error(fmt.Sprintf("Could not connect to %v", ec.raddr), err)
		return err
	}
	ec.conn = conn
	logger.Info(fmt.Sprintf("Connected to %v.", ec.raddr))
	return nil
}

func (ec *EmberClient) Disconnect() error {
	if !ec.IsConnected() {
		return errors.New("not connected")
	} else {
		logger.Info(fmt.Sprintf("Disconnecting from %v.", ec.raddr))
		ec.conn.Close()
		logger.Info(fmt.Sprintf("Disconnected from %v.", ec.raddr))
		return nil
	}
}

func (ec *EmberClient) sendBerData(data []byte) error {
	if !ec.IsConnected() {
		return errors.New("not connected")
	}
	_ = data
	return nil
}
