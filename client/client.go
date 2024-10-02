package client

import "fmt"

type EmberClient struct {
	//conn net.Conn
	IsConnected bool
	Addr        string
}

func NewEmberClient(address string, port uint) *EmberClient {
	var ec EmberClient
	ec.Addr = fmt.Sprintf("%s:%d", address, port)
	return &ec
}
