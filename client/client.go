package client

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/johannes-kuhfuss/emberplus/asn1"
	"github.com/johannes-kuhfuss/emberplus/ember"
	"github.com/johannes-kuhfuss/emberplus/s101"
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
		logger.Info(fmt.Sprintf("Disconnecting from %v...", ec.raddr))
		ec.conn.Close()
		logger.Info(fmt.Sprintf("Disconnected from %v.", ec.raddr))
		return nil
	}
}

func (ec *EmberClient) Write(data []byte) (int, error) {
	if !ec.IsConnected() {
		return 0, errors.New("not connected")
	} else {
		n, err := ec.conn.Write(data)
		if err != nil {
			return 0, fmt.Errorf("error writing bytes: %w", err)
		}
		return n, nil
	}
}

func (ec *EmberClient) Receive() ([]byte, error) {
	var (
		s101s          [][]byte
		incompleteS101 []byte
		out            []byte
		multi          bool
	)
	if !ec.IsConnected() {
		return nil, errors.New("not connected")
	} else {
		for {
			response := make([]byte, 1290)
			n, err := ec.conn.Read(response)
			if err != nil {
				return nil, fmt.Errorf("failed to read from connection: %w", err)
			}

			if len(incompleteS101) > 0 {
				response = append(incompleteS101, response[:n]...)
			}

			s101s, incompleteS101, err = s101.GetS101s(response)
			if err != nil {
				return nil, fmt.Errorf("failed to get s101 data from read: %w", err)
			}

			if len(incompleteS101) > 0 {
				continue
			}

			glow, lastPacketType, err := s101.Decode(s101s)
			if err != nil {
				logger.Debug(fmt.Sprintf("failed to decode response: %s", err.Error()))
				continue
			}
			switch lastPacketType {
			case s101.FirstMultiPacket, s101.BodyMultiPacket:
				out = append(out, glow...)
				multi = true
				continue
			case s101.LastMultiPacket:
				out = append(out, glow...)
				return out, nil
			default:
				if multi {
					logger.Error(fmt.Sprintf("dropping message in the middle of a multi packet read %x", glow), err)
					continue
				}
				return glow, nil
			}
		}
	}
}

func (ec *EmberClient) GetRoot() ([]byte, error) {
	data, err := ec.GetByType("qualified_node", "")
	if err != nil {
		logger.Error("error getting root request.", err)
		return nil, err
	}
	return data, nil
}

func (ec *EmberClient) GetByType(emberType ember.ElementType, emberPath string) ([]byte, error) {
	if !ec.IsConnected() {
		return nil, errors.New("not connected")
	} else {
		tr, err := ember.GetRequestByType(emberType, emberPath)
		if err != nil {
			logger.Error(fmt.Sprintf("error getting request. Type: %v, Path: %v", emberType, emberPath), err)
			return nil, err
		}
		ec.Write(tr)
		out, err := ec.Receive()
		if err != nil {
			logger.Error(fmt.Sprintf("error getting asnwer. Type: %v, Path: %v", emberType, emberPath), err)
			return nil, err
		}
		el2 := ember.NewElementConnection()
		err = el2.Populate(asn1.NewDecoder(out))
		if err != nil {
			logger.Error(fmt.Sprintf("error processing answer. Type: %v, Path: %v", emberType, emberPath), err)
			return nil, err
		}
		data, err := el2.MarshalJSON()
		if err != nil {
			logger.Error(fmt.Sprintf("error marshalling answer to JSON. Type: %v, Path: %v", emberType, emberPath), err)
			return nil, err
		}
		return data, nil
	}
}
