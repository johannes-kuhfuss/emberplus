package emberclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/johannes-kuhfuss/emberplus/asn1"
	"github.com/johannes-kuhfuss/emberplus/ember"
	"github.com/johannes-kuhfuss/emberplus/s101"
)

const defaultTimeout = 30 * time.Second

type EmberClient struct {
	raddr        string
	conn         net.Conn
	timeout      time.Duration
	decoder      *s101.StreamDecoder
	frames       []s101.Frame
	pendingRoots []ember.RootMessage
	requestMu    sync.Mutex
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
	ec.decoder = s101.NewStreamDecoder()
	ec.frames = nil
	ec.pendingRoots = nil
	return nil
}

func (ec *EmberClient) Disconnect() error {
	if !ec.IsConnected() {
		return errors.New("not connected")
	}
	err := ec.conn.Close()
	ec.conn = nil
	ec.decoder = nil
	ec.frames = nil
	ec.pendingRoots = nil
	if err != nil {
		return fmt.Errorf("failed to disconnect from %s: %w", ec.raddr, err)
	}
	return nil
}

func (ec *EmberClient) Write(data []byte) (int, error) {
	return ec.WriteContext(context.Background(), data)
}

// WriteContext writes all data while honoring both the client timeout and context cancellation.
func (ec *EmberClient) WriteContext(ctx context.Context, data []byte) (int, error) {
	if !ec.IsConnected() {
		return 0, errors.New("not connected")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if err := ec.setWriteDeadline(ctx); err != nil {
		return 0, err
	}
	stop := ec.interruptOnCancel(ctx, false)
	defer stop()

	n, err := ec.conn.Write(data)
	if err != nil {
		if ctxErr := contextError(ctx); ctxErr != nil {
			return n, ctxErr
		}
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
		out   []byte
		multi bool
	)
	if !ec.IsConnected() {
		return nil, errors.New("not connected")
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		frame, err := ec.nextFrame(ctx)
		if err != nil {
			return nil, err
		}
		if frame.Command == s101.CommandKeepAliveRequest {
			response, err := s101.EncodeFrame(s101.Frame{Framing: frame.Framing, Slot: frame.Slot, Command: s101.CommandKeepAliveResponse})
			if err != nil {
				return nil, err
			}
			if _, err := ec.WriteContext(ctx, response); err != nil {
				return nil, fmt.Errorf("failed to answer S101 keep-alive: %w", err)
			}
			continue
		}
		if frame.Command == s101.CommandKeepAliveResponse {
			continue
		}
		switch frame.Flags {
		case s101.FirstMultiPacket, s101.BodyMultiPacket:
			out = append(out, frame.Payload...)
			multi = true
			continue
		case s101.LastMultiPacket:
			out = append(out, frame.Payload...)
			return out, nil
		case s101.SinglePacket:
			if multi {
				return nil, errors.New("dropping message in the middle of a multi packet read")
			}
			return frame.Payload, nil
		default:
			return nil, fmt.Errorf("invalid S101 packet flag: %x", frame.Flags)
		}
	}
}

func (ec *EmberClient) nextFrame(ctx context.Context) (s101.Frame, error) {
	for {
		if len(ec.frames) > 0 {
			frame := ec.frames[0]
			ec.frames = ec.frames[1:]
			return frame, nil
		}
		if ec.decoder == nil {
			ec.decoder = s101.NewStreamDecoder()
		}
		if err := ec.setReadDeadline(ctx); err != nil {
			return s101.Frame{}, err
		}
		stop := ec.interruptOnCancel(ctx, true)
		response := make([]byte, 4096)
		n, err := ec.conn.Read(response)
		stop()
		if err != nil {
			if ctxErr := contextError(ctx); ctxErr != nil {
				return s101.Frame{}, ctxErr
			}
			return s101.Frame{}, fmt.Errorf("failed to read from connection: %w", err)
		}
		frames, err := ec.decoder.Push(response[:n])
		if err != nil {
			return s101.Frame{}, fmt.Errorf("failed to decode S101 stream: %w", err)
		}
		ec.frames = append(ec.frames, frames...)
	}
}

func (ec *EmberClient) GetRoot() ([]byte, error) {
	return ec.GetByType(ember.QualifiedNodeElement, "")
}

func (ec *EmberClient) GetRootCollection() (ember.ElementCollection, error) {
	return ec.GetElementCollection(ember.QualifiedNodeElement, "")
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
	ec.requestMu.Lock()
	defer ec.requestMu.Unlock()

	if !ec.IsConnected() {
		return nil, errors.New("not connected")
	}

	glow, err := ember.EncodeGetDirectory(emberType, emberPath, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to get Ember request. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}
	tr := s101.Encode(glow, s101.SinglePacket)
	if _, err = ec.WriteContext(ctx, tr); err != nil {
		return nil, fmt.Errorf("failed to write Ember request. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}

	out, err := ec.ReceiveContext(ctx)
	if err != nil {
		_ = ec.closeConn()
		return nil, fmt.Errorf("failed to get Ember answer. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}

	collection := ember.NewElementCollection()
	if err = collection.PopulateGlow250(asn1.NewDecoder(out)); err != nil {
		return nil, fmt.Errorf("failed to process Ember answer. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}

	return collection, nil
}

// ReceiveRootContext reads the next complete Glow root message, including unsolicited notifications and streams.
func (ec *EmberClient) ReceiveRootContext(ctx context.Context) (ember.RootMessage, error) {
	if len(ec.pendingRoots) > 0 {
		message := ec.pendingRoots[0]
		ec.pendingRoots = ec.pendingRoots[1:]
		return message, nil
	}
	return ec.receiveRootDirect(ctx)
}

// ReceiveRoot reads the next complete Glow root message.
func (ec *EmberClient) ReceiveRoot() (ember.RootMessage, error) {
	return ec.ReceiveRootContext(context.Background())
}

func (ec *EmberClient) receiveRootDirect(ctx context.Context) (ember.RootMessage, error) {
	data, err := ec.ReceiveContext(ctx)
	if err != nil {
		return ember.RootMessage{}, err
	}
	message, err := ember.DecodeRoot(data)
	if err != nil {
		return ember.RootMessage{}, fmt.Errorf("failed to decode Glow root: %w", err)
	}
	return message, nil
}

// SetParameter changes a parameter value and returns the provider's response.
func (ec *EmberClient) SetParameter(ctx context.Context, path string, value any) (ember.RootMessage, error) {
	glow, err := ember.EncodeSetParameter(path, value)
	if err != nil {
		return ember.RootMessage{}, err
	}
	return ec.exchange(ctx, glow)
}

// SetMatrixConnections changes one or more matrix connections.
func (ec *EmberClient) SetMatrixConnections(ctx context.Context, path string, connections []ember.MatrixConnection) (ember.RootMessage, error) {
	glow, err := ember.EncodeMatrixConnections(path, connections)
	if err != nil {
		return ember.RootMessage{}, err
	}
	return ec.exchange(ctx, glow)
}

// Invoke calls a Glow function and waits for its invocation result.
func (ec *EmberClient) Invoke(ctx context.Context, path string, id int64, arguments ...any) (*ember.InvocationResult, error) {
	glow, err := ember.EncodeInvocation(path, id, arguments)
	if err != nil {
		return nil, err
	}
	ec.requestMu.Lock()
	defer ec.requestMu.Unlock()
	if _, err := ec.WriteContext(ctx, s101.Encode(glow, s101.SinglePacket)); err != nil {
		return nil, err
	}
	for {
		message, err := ec.receiveRootDirect(ctx)
		if err != nil {
			return nil, err
		}
		if message.InvocationResult != nil && (id == 0 || message.InvocationResult.ID == id) {
			return message.InvocationResult, nil
		}
		ec.pendingRoots = append(ec.pendingRoots, message)
	}
}

// Subscribe changes the subscription state of a parameter.
func (ec *EmberClient) Subscribe(ctx context.Context, elementType ember.ElementType, path string, subscribe bool) error {
	glow, err := ember.EncodeSubscription(elementType, path, subscribe)
	if err != nil {
		return err
	}
	ec.requestMu.Lock()
	defer ec.requestMu.Unlock()
	_, err = ec.WriteContext(ctx, s101.Encode(glow, s101.SinglePacket))
	return err
}

// KeepAlive sends an S101 keep-alive request and waits for the response.
func (ec *EmberClient) KeepAlive(ctx context.Context) error {
	ec.requestMu.Lock()
	defer ec.requestMu.Unlock()
	request, err := s101.EncodeKeepAlive(s101.CommandKeepAliveRequest)
	if err != nil {
		return err
	}
	if _, err := ec.WriteContext(ctx, request); err != nil {
		return err
	}
	var deferred []s101.Frame
	defer func() { ec.frames = append(deferred, ec.frames...) }()
	for {
		frame, err := ec.nextFrame(ctx)
		if err != nil {
			return err
		}
		switch frame.Command {
		case s101.CommandKeepAliveResponse:
			return nil
		case s101.CommandKeepAliveRequest:
			response, err := s101.EncodeFrame(s101.Frame{Framing: frame.Framing, Slot: frame.Slot, Command: s101.CommandKeepAliveResponse})
			if err != nil {
				return err
			}
			if _, err := ec.WriteContext(ctx, response); err != nil {
				return err
			}
		default:
			deferred = append(deferred, frame)
		}
	}
}

// Serve continuously reads notifications, streams, and invocation results.
// Keeping Serve active also guarantees that peer keep-alive requests are answered while the client is otherwise idle.
// Do not call request methods concurrently with Serve; use one goroutine as the connection owner.
func (ec *EmberClient) Serve(ctx context.Context, handler func(ember.RootMessage) error) error {
	if handler == nil {
		return errors.New("nil Glow message handler")
	}
	for {
		message, err := ec.ReceiveRootContext(ctx)
		if err != nil {
			return err
		}
		if err := handler(message); err != nil {
			return err
		}
	}
}

func (ec *EmberClient) exchange(ctx context.Context, glow []byte) (ember.RootMessage, error) {
	ec.requestMu.Lock()
	defer ec.requestMu.Unlock()
	if !ec.IsConnected() {
		return ember.RootMessage{}, errors.New("not connected")
	}
	if _, err := ec.WriteContext(ctx, s101.Encode(glow, s101.SinglePacket)); err != nil {
		return ember.RootMessage{}, err
	}
	// A queued root is an earlier unsolicited message. The response to this
	// request must come from the wire so notification delivery stays ordered.
	return ec.receiveRootDirect(ctx)
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

func (ec *EmberClient) setReadDeadline(ctx context.Context) error {
	deadline, ok := ec.operationDeadline(ctx)
	if !ok {
		return ec.conn.SetReadDeadline(time.Time{})
	}
	if err := ec.conn.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set connection read deadline: %w", err)
	}
	return nil
}

func (ec *EmberClient) setWriteDeadline(ctx context.Context) error {
	if ec.timeout > 0 {
		if err := ec.setDeadline(time.Now()); err != nil {
			return err
		}
	}
	deadline, ok := ec.operationDeadline(ctx)
	if !ok {
		return ec.conn.SetWriteDeadline(time.Time{})
	}
	if err := ec.conn.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set connection write deadline: %w", err)
	}
	return nil
}

func (ec *EmberClient) operationDeadline(ctx context.Context) (time.Time, bool) {
	var deadline time.Time
	if ec.timeout > 0 {
		deadline = time.Now().Add(ec.timeout)
	}
	if contextDeadline, ok := ctx.Deadline(); ok && (deadline.IsZero() || contextDeadline.Before(deadline)) {
		deadline = contextDeadline
	}
	return deadline, !deadline.IsZero()
}

func (ec *EmberClient) interruptOnCancel(ctx context.Context, read bool) func() {
	if ctx.Done() == nil {
		return func() {}
	}
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if read {
				_ = ec.conn.SetReadDeadline(time.Now())
			} else {
				_ = ec.conn.SetWriteDeadline(time.Now())
			}
		case <-stopped:
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(stopped) }) }
}

func contextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

func (ec *EmberClient) closeConn() error {
	if ec.conn == nil {
		return nil
	}

	err := ec.conn.Close()
	ec.conn = nil
	ec.decoder = nil
	ec.frames = nil
	ec.pendingRoots = nil

	return err
}
