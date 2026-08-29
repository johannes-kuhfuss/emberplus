package emberclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/johannes-kuhfuss/emberplus/asn1"
	"github.com/johannes-kuhfuss/emberplus/ember"
	"github.com/johannes-kuhfuss/emberplus/s101"
)

const defaultTimeout = 30 * time.Second

// ErrReceiveActive indicates that another operation currently owns the connection reader.
var ErrReceiveActive = errors.New("connection receive loop is already active")

type EmberClient struct {
	raddr        string
	conn         net.Conn
	timeout      time.Duration
	decoder      *s101.StreamDecoder
	frames       []s101.Frame
	pendingRoots []ember.RootMessage
	stateMu      sync.Mutex
	readerMu     sync.Mutex
	writeMu      sync.Mutex
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
// Serve reads until its context is cancelled and does not use this timeout.
// A zero or negative duration disables the timeout.
func (ec *EmberClient) SetTimeout(timeout time.Duration) {
	ec.stateMu.Lock()
	defer ec.stateMu.Unlock()
	ec.timeout = timeout
}

func (ec *EmberClient) IsConnected() bool {
	ec.stateMu.Lock()
	defer ec.stateMu.Unlock()
	return ec.conn != nil
}

func (ec *EmberClient) Connect() error {
	return ec.ConnectContext(context.Background())
}

func (ec *EmberClient) ConnectContext(ctx context.Context) error {
	if !ec.readerMu.TryLock() {
		return ErrReceiveActive
	}
	defer ec.readerMu.Unlock()
	ec.stateMu.Lock()
	defer ec.stateMu.Unlock()
	if ec.conn != nil {
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
	ec.stateMu.Lock()
	if ec.conn == nil {
		ec.stateMu.Unlock()
		return errors.New("not connected")
	}
	conn := ec.conn
	ec.conn = nil
	ec.decoder = nil
	ec.frames = nil
	ec.pendingRoots = nil
	ec.stateMu.Unlock()

	err := conn.Close()
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
	conn, timeout := ec.connection()
	if conn == nil {
		return 0, errors.New("not connected")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	ec.writeMu.Lock()
	defer ec.writeMu.Unlock()

	if err := setWriteDeadline(conn, timeout, ctx); err != nil {
		_ = ec.closeConnIf(conn)
		return 0, err
	}
	stop := interruptOnCancel(conn, ctx, false)
	defer stop()

	n, err := conn.Write(data)
	if err != nil {
		if ctxErr := contextError(ctx); ctxErr != nil {
			return n, ctxErr
		}
		_ = ec.closeConnIf(conn)
		return n, fmt.Errorf("error writing bytes: %w", err)
	}
	if n != len(data) {
		_ = ec.closeConnIf(conn)
		return n, io.ErrShortWrite
	}

	return n, nil
}

func (ec *EmberClient) Receive() ([]byte, error) {
	return ec.ReceiveContext(context.Background())
}

func (ec *EmberClient) ReceiveContext(ctx context.Context) ([]byte, error) {
	if !ec.readerMu.TryLock() {
		return nil, ErrReceiveActive
	}
	defer ec.readerMu.Unlock()
	return ec.receiveContext(ctx, true)
}

func (ec *EmberClient) receiveContext(ctx context.Context, useClientTimeout bool) ([]byte, error) {
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
		frame, err := ec.nextFrame(ctx, useClientTimeout)
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

func (ec *EmberClient) nextFrame(ctx context.Context, useClientTimeout bool) (s101.Frame, error) {
	for {
		ec.stateMu.Lock()
		if len(ec.frames) > 0 {

			queueLen := len(ec.frames)

			log.Printf(
				"[EMBER DEBUG] consuming queued frame: queue=%d",
				queueLen,
			)

			frame := ec.frames[0]
			ec.frames = ec.frames[1:]
			ec.stateMu.Unlock()
			return frame, nil
		}
		if ec.decoder == nil {
			ec.decoder = s101.NewStreamDecoder()
		}
		decoder := ec.decoder
		conn := ec.conn
		timeout := ec.timeout
		ec.stateMu.Unlock()
		if conn == nil {
			return s101.Frame{}, errors.New("not connected")
		}
		if !useClientTimeout {
			timeout = 0
		}
		if err := setReadDeadline(conn, timeout, ctx); err != nil {
			_ = ec.closeConnIf(conn)
			return s101.Frame{}, err
		}
		stop := interruptOnCancel(conn, ctx, true)
		response := make([]byte, 4096)
		n, err := conn.Read(response)
		stop()
		if err != nil {
			if ctxErr := contextError(ctx); ctxErr != nil {
				return s101.Frame{}, ctxErr
			}
			_ = ec.closeConnIf(conn)
			return s101.Frame{}, fmt.Errorf("failed to read from connection: %w", err)
		}
		frames, err := decoder.Push(response[:n])
		if err != nil {
			_ = ec.closeConnIf(conn)
			return s101.Frame{}, fmt.Errorf("failed to decode S101 stream: %w", err)
		}
		ec.stateMu.Lock()
		before := len(ec.frames)
		ec.frames = append(ec.frames, frames...)
		after := len(ec.frames)

		log.Printf(
			"[EMBER DEBUG] TCP read=%d bytes decoded=%d frames queue=%d->%d",
			n,
			len(frames),
			before,
			after,
		)

		ec.stateMu.Unlock()
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
	return ec.getElementCollectionContext(ctx, emberType, emberPath, false)
}

// GetElementCollectionGlow250 returns the complete Glow 2.50 value representation.
func (ec *EmberClient) GetElementCollectionGlow250(emberType ember.ElementType, emberPath string) (ember.ElementCollection, error) {
	return ec.GetElementCollectionGlow250Context(context.Background(), emberType, emberPath)
}

// GetElementCollectionGlow250Context returns the complete Glow 2.50 value representation.
func (ec *EmberClient) GetElementCollectionGlow250Context(
	ctx context.Context, emberType ember.ElementType, emberPath string,
) (ember.ElementCollection, error) {
	return ec.getElementCollectionContext(ctx, emberType, emberPath, true)
}

func (ec *EmberClient) getElementCollectionContext(
	ctx context.Context, emberType ember.ElementType, emberPath string, glow250 bool,
) (ember.ElementCollection, error) {
	ec.requestMu.Lock()
	defer ec.requestMu.Unlock()
	if !ec.readerMu.TryLock() {
		return nil, ErrReceiveActive
	}
	defer ec.readerMu.Unlock()

	if !ec.IsConnected() {
		return nil, errors.New("not connected")
	}

	glow, err := ember.EncodeGetDirectory(emberType, emberPath, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to get Ember request. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}
	tr := s101.Encode(glow, s101.FirstMultiPacket)
	if _, err = ec.WriteContext(ctx, tr); err != nil {
		return nil, fmt.Errorf("failed to write Ember request. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}

	ec.stateMu.Lock()
	queued := len(ec.frames)
	ec.stateMu.Unlock()

	log.Printf(
		"[EMBER DEBUG] GetDirectory type=%v path=%q queued-before-receive=%d",
		emberType,
		emberPath,
		queued,
	)

	out, err := ec.receiveContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get Ember answer. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}

	collection := ember.NewElementCollection()
	if glow250 {
		err = collection.PopulateGlow250(asn1.NewDecoder(out))
	} else {
		err = collection.Populate(asn1.NewDecoder(out))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to process Ember answer. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}

	return collection, nil
}

// ReceiveRootContext reads the next complete Glow root message, including unsolicited notifications and streams.
func (ec *EmberClient) ReceiveRootContext(ctx context.Context) (ember.RootMessage, error) {
	if !ec.readerMu.TryLock() {
		return ember.RootMessage{}, ErrReceiveActive
	}
	defer ec.readerMu.Unlock()
	if message, ok := ec.popPendingRoot(); ok {
		return message, nil
	}
	return ec.receiveRootDirect(ctx, true)
}

// ReceiveRoot reads the next complete Glow root message.
func (ec *EmberClient) ReceiveRoot() (ember.RootMessage, error) {
	return ec.ReceiveRootContext(context.Background())
}

func (ec *EmberClient) receiveRootDirect(ctx context.Context, useClientTimeout bool) (ember.RootMessage, error) {
	data, err := ec.receiveContext(ctx, useClientTimeout)
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
	if !ec.readerMu.TryLock() {
		return nil, ErrReceiveActive
	}
	defer ec.readerMu.Unlock()
	if _, err := ec.WriteContext(ctx, s101.Encode(glow, s101.SinglePacket)); err != nil {
		return nil, err
	}
	for {
		message, err := ec.receiveRootDirect(ctx, true)
		if err != nil {
			return nil, err
		}
		if message.InvocationResult != nil && (id == 0 || message.InvocationResult.ID == id) {
			return message.InvocationResult, nil
		}
		ec.pushPendingRoot(message)
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
	if !ec.readerMu.TryLock() {
		return ErrReceiveActive
	}
	defer ec.readerMu.Unlock()
	_, err = ec.WriteContext(ctx, s101.Encode(glow, s101.SinglePacket))
	return err
}

// KeepAlive sends an S101 keep-alive request and waits for the response.
func (ec *EmberClient) KeepAlive(ctx context.Context) error {
	ec.requestMu.Lock()
	defer ec.requestMu.Unlock()
	if !ec.readerMu.TryLock() {
		return ErrReceiveActive
	}
	defer ec.readerMu.Unlock()
	request, err := s101.EncodeKeepAlive(s101.CommandKeepAliveRequest)
	if err != nil {
		return err
	}
	if _, err := ec.WriteContext(ctx, request); err != nil {
		return err
	}
	var deferred []s101.Frame
	defer func() {
		ec.stateMu.Lock()
		ec.frames = append(deferred, ec.frames...)
		ec.stateMu.Unlock()
	}()
	for {
		frame, err := ec.nextFrame(ctx, true)
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
	if !ec.readerMu.TryLock() {
		return ErrReceiveActive
	}
	defer ec.readerMu.Unlock()
	for {
		var (
			message ember.RootMessage
			err     error
		)
		if pending, ok := ec.popPendingRoot(); ok {
			message = pending
		} else {
			message, err = ec.receiveRootDirect(ctx, false)
		}
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
	if !ec.readerMu.TryLock() {
		return ember.RootMessage{}, ErrReceiveActive
	}
	defer ec.readerMu.Unlock()
	if !ec.IsConnected() {
		return ember.RootMessage{}, errors.New("not connected")
	}
	if _, err := ec.WriteContext(ctx, s101.Encode(glow, s101.SinglePacket)); err != nil {
		return ember.RootMessage{}, err
	}
	// A queued root is an earlier unsolicited message. The response to this
	// request must come from the wire so notification delivery stays ordered.
	return ec.receiveRootDirect(ctx, true)
}

func setReadDeadline(conn net.Conn, timeout time.Duration, ctx context.Context) error {
	deadline, ok := operationDeadline(timeout, ctx)
	if !ok {
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return fmt.Errorf("failed to clear connection read deadline: %w", err)
		}
		return nil
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set connection read deadline: %w", err)
	}
	return nil
}

func setWriteDeadline(conn net.Conn, timeout time.Duration, ctx context.Context) error {
	deadline, ok := operationDeadline(timeout, ctx)
	if !ok {
		if err := conn.SetWriteDeadline(time.Time{}); err != nil {
			return fmt.Errorf("failed to clear connection write deadline: %w", err)
		}
		return nil
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set connection write deadline: %w", err)
	}
	return nil
}

func operationDeadline(timeout time.Duration, ctx context.Context) (time.Time, bool) {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if contextDeadline, ok := ctx.Deadline(); ok && (deadline.IsZero() || contextDeadline.Before(deadline)) {
		deadline = contextDeadline
	}
	return deadline, !deadline.IsZero()
}

func interruptOnCancel(conn net.Conn, ctx context.Context, read bool) func() {
	if ctx.Done() == nil {
		return func() {}
	}
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if read {
				_ = conn.SetReadDeadline(time.Now())
			} else {
				_ = conn.SetWriteDeadline(time.Now())
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

func (ec *EmberClient) connection() (net.Conn, time.Duration) {
	ec.stateMu.Lock()
	defer ec.stateMu.Unlock()
	return ec.conn, ec.timeout
}

func (ec *EmberClient) closeConnIf(conn net.Conn) error {
	ec.stateMu.Lock()
	if ec.conn != conn {
		ec.stateMu.Unlock()
		return nil
	}
	ec.conn = nil
	ec.decoder = nil
	ec.frames = nil
	ec.pendingRoots = nil
	ec.stateMu.Unlock()
	return conn.Close()
}

func (ec *EmberClient) popPendingRoot() (ember.RootMessage, bool) {
	ec.stateMu.Lock()
	defer ec.stateMu.Unlock()
	if len(ec.pendingRoots) == 0 {
		return ember.RootMessage{}, false
	}
	message := ec.pendingRoots[0]
	ec.pendingRoots = ec.pendingRoots[1:]
	return message, true
}

func (ec *EmberClient) pushPendingRoot(message ember.RootMessage) {
	ec.stateMu.Lock()
	defer ec.stateMu.Unlock()
	ec.pendingRoots = append(ec.pendingRoots, message)
}
