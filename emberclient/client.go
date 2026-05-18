package emberclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/johannes-kuhfuss/emberplus/asn1"
	"github.com/johannes-kuhfuss/emberplus/ember"
	"github.com/johannes-kuhfuss/emberplus/s101"
)

const defaultTimeout = 30 * time.Second

type EmberClient struct {
	raddr   string
	conn    net.Conn
	timeout time.Duration
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
	ec.timeout = defaultTimeout
	return &ec, nil
}

// SetTimeout configures the per-operation timeout for connect, read, and write calls.
// A zero or negative duration disables the timeout.
func (ec *EmberClient) SetTimeout(timeout time.Duration) {
	ec.timeout = timeout
}

func (ec *EmberClient) IsConnected() bool {
	return ec.conn != nil
}

func (ec *EmberClient) Connect() error {
	return ec.ConnectContext(context.Background())
}

func (ec *EmberClient) ConnectContext(ctx context.Context) error {
	if ec.IsConnected() {
		return errors.New("already connected")
	}

	dialer := net.Dialer{Timeout: ec.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", ec.raddr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", ec.raddr, err)
	}
	ec.conn = conn
	return nil
}

func (ec *EmberClient) Disconnect() error {
	if !ec.IsConnected() {
		return errors.New("not connected")
	}
	err := ec.conn.Close()
	ec.conn = nil
	if err != nil {
		return fmt.Errorf("failed to disconnect from %s: %w", ec.raddr, err)
	}
	return nil
}

func (ec *EmberClient) Write(data []byte) (int, error) {
	if !ec.IsConnected() {
		return 0, errors.New("not connected")
	}

	if err := ec.setDeadline(time.Now()); err != nil {
		return 0, err
	}

	n, err := ec.conn.Write(data)
	if err != nil {
		return n, fmt.Errorf("error writing bytes: %w", err)
	}
	if n != len(data) {
		return n, io.ErrShortWrite
	}

	return n, nil
}

func (ec *EmberClient) Receive() ([]byte, error) {
	return ec.ReceiveContext(context.Background())
}

func (ec *EmberClient) ReceiveContext(ctx context.Context) ([]byte, error) {
	var (
		s101s          [][]byte
		incompleteS101 []byte
		out            []byte
		multi          bool
	)
	if !ec.IsConnected() {
		return nil, errors.New("not connected")
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := ec.setDeadline(time.Now()); err != nil {
			return nil, err
		}

		response := make([]byte, 1290)
		n, err := ec.conn.Read(response)
		if err != nil {
			return nil, fmt.Errorf("failed to read from connection: %w", err)
		}

		response = response[:n]
		if len(incompleteS101) > 0 {
			response = append(incompleteS101, response...)
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
			return nil, fmt.Errorf("failed to decode response: %w", err)
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
				return nil, errors.New("dropping message in the middle of a multi packet read")
			}
			return glow, nil
		}
	}
}

func (ec *EmberClient) GetRoot() ([]byte, error) {
	return ec.GetByType("qualified_node", "")
}

func (ec *EmberClient) GetRootCollection() (ember.ElementCollection, error) {
	return ec.GetElementCollection("qualified_node", "")
}

func (ec *EmberClient) GetByType(emberType ember.ElementType, emberPath string) ([]byte, error) {
	return ec.GetByTypeContext(context.Background(), emberType, emberPath)
}

func (ec *EmberClient) GetByTypeContext(ctx context.Context, emberType ember.ElementType, emberPath string) ([]byte, error) {
	collection, err := ec.GetElementCollectionContext(ctx, emberType, emberPath)
	if err != nil {
		return nil, err
	}

	data, err := collection.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Ember answer to JSON. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}

	return data, nil
}

func (ec *EmberClient) GetElementCollection(emberType ember.ElementType, emberPath string) (ember.ElementCollection, error) {
	return ec.GetElementCollectionContext(context.Background(), emberType, emberPath)
}

func (ec *EmberClient) GetElementCollectionContext(ctx context.Context, emberType ember.ElementType, emberPath string) (ember.ElementCollection, error) {
	if !ec.IsConnected() {
		return nil, errors.New("not connected")
	}

	tr, err := ember.GetRequestByType(emberType, emberPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get Ember request. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}
	if _, err = ec.Write(tr); err != nil {
		return nil, fmt.Errorf("failed to write Ember request. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}

	out, err := ec.ReceiveContext(ctx)
	if err != nil {
		_ = ec.closeConn()
		return nil, fmt.Errorf("failed to get Ember answer. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}

	collection := ember.NewElementConnection()
	if err = collection.Populate(asn1.NewDecoder(out)); err != nil {
		return nil, fmt.Errorf("failed to process Ember answer. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}

	return collection, nil
}

func (ec *EmberClient) setDeadline(now time.Time) error {
	if ec.timeout <= 0 {
		return nil
	}

	if err := ec.conn.SetDeadline(now.Add(ec.timeout)); err != nil {
		return fmt.Errorf("failed to set connection deadline: %w", err)
	}

	return nil
}

func (ec *EmberClient) closeConn() error {
	if ec.conn == nil {
		return nil
	}

	err := ec.conn.Close()
	ec.conn = nil

	return err
}
