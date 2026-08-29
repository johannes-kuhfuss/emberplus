package emberclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/johannes-kuhfuss/emberplus/asn1"
	"github.com/johannes-kuhfuss/emberplus/ember"
	"github.com/johannes-kuhfuss/emberplus/s101"
)

const (
	defaultTimeout            = 30 * time.Second
	defaultNotificationBuffer = 256

	DiagnosticRequestBacklog       = "request_backlog"
	DiagnosticNotificationOverflow = "notification_overflow"
	DiagnosticReadPumpError        = "read_pump_error"
)

// Diagnostic describes an exceptional receive-pump condition. Diagnostics are
// disabled unless SetDiagnosticHandler is called.
type Diagnostic struct {
	Kind                 string
	Request              string
	Path                 string
	SkippedRoots         int
	DroppedNotifications uint64
	Duration             time.Duration
	Err                  error
}

type receivedMessage struct {
	raw     []byte
	root    ember.RootMessage
	rootErr error
}

type responseWaiter struct {
	request string
	path    string
	started time.Time
	skipped int
	match   func(ember.RootMessage) bool
	result  chan requestResult
}

type requestResult struct {
	message receivedMessage
	err     error
}

// EmberClient owns one permanent socket read pump. Request methods only write
// and wait for messages routed to them by that pump.
type EmberClient struct {
	raddr   string
	conn    net.Conn
	timeout time.Duration

	stateMu   sync.Mutex
	writeMu   sync.Mutex
	requestMu sync.Mutex

	waiter          *responseWaiter
	keepAliveWaiter chan error
	notifications   chan receivedMessage
	latestElements  map[string]*ember.Element
	dropped         uint64
	pumpCancel      context.CancelFunc
	pumpDone        chan struct{}
	pumpErr         error
	diagnostic      func(Diagnostic)
}

func NewEmberClient(host string, port int) (*EmberClient, error) {
	if port < 1 || port > 65535 {
		return nil, errors.New("port must be between 1 and 65535")
	}
	if host == "" {
		return nil, errors.New("host must be either a host name or an IP address")
	}
	return &EmberClient{raddr: net.JoinHostPort(host, strconv.Itoa(port)), timeout: defaultTimeout}, nil
}

// SetDiagnosticHandler installs an optional handler for backlog, overflow, and
// read-pump errors. The handler must return promptly.
func (ec *EmberClient) SetDiagnosticHandler(handler func(Diagnostic)) {
	ec.stateMu.Lock()
	ec.diagnostic = handler
	ec.stateMu.Unlock()
}

func (ec *EmberClient) emit(event Diagnostic) {
	ec.stateMu.Lock()
	handler := ec.diagnostic
	ec.stateMu.Unlock()
	if handler != nil {
		handler(event)
	}
}

func (ec *EmberClient) SetTimeout(timeout time.Duration) {
	ec.stateMu.Lock()
	ec.timeout = timeout
	ec.stateMu.Unlock()
}

func (ec *EmberClient) IsConnected() bool {
	ec.stateMu.Lock()
	defer ec.stateMu.Unlock()
	return ec.conn != nil
}

func (ec *EmberClient) Connect() error { return ec.ConnectContext(context.Background()) }

func (ec *EmberClient) ConnectContext(ctx context.Context) error {
	ec.stateMu.Lock()
	if ec.conn != nil {
		ec.stateMu.Unlock()
		return errors.New("already connected")
	}
	timeout := ec.timeout
	ec.stateMu.Unlock()

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", ec.raddr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", ec.raddr, err)
	}
	ec.stateMu.Lock()
	if ec.conn != nil {
		ec.stateMu.Unlock()
		_ = conn.Close()
		return errors.New("already connected")
	}
	ec.startPumpLocked(conn)
	ec.stateMu.Unlock()
	return nil
}

func (ec *EmberClient) startPumpLocked(conn net.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	notifications := make(chan receivedMessage, defaultNotificationBuffer)
	done := make(chan struct{})
	ec.conn = conn
	ec.notifications = notifications
	ec.latestElements = make(map[string]*ember.Element)
	ec.dropped = 0
	ec.pumpErr = nil
	ec.pumpCancel = cancel
	ec.pumpDone = done
	go ec.readPump(ctx, conn, notifications, done)
}

func (ec *EmberClient) ensurePump() error {
	ec.stateMu.Lock()
	defer ec.stateMu.Unlock()
	if ec.conn == nil {
		return errors.New("not connected")
	}
	if ec.pumpDone == nil {
		ec.startPumpLocked(ec.conn)
	}
	return nil
}

func (ec *EmberClient) Disconnect() error {
	ec.stateMu.Lock()
	if ec.conn == nil {
		ec.stateMu.Unlock()
		return errors.New("not connected")
	}
	conn, done, cancel := ec.conn, ec.pumpDone, ec.pumpCancel
	ec.conn = nil
	ec.failWaitersLocked(errors.New("disconnected"))
	ec.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	err := conn.Close()
	if done != nil {
		<-done
	}
	if err != nil {
		return fmt.Errorf("failed to disconnect from %s: %w", ec.raddr, err)
	}
	return nil
}

func (ec *EmberClient) Write(data []byte) (int, error) {
	return ec.WriteContext(context.Background(), data)
}

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
		ec.stopConnection(conn, err)
		return 0, err
	}
	stop := interruptOnCancel(conn, ctx, false)
	defer stop()
	n, err := conn.Write(data)
	if err != nil {
		if ctxErr := contextError(ctx); ctxErr != nil {
			return n, ctxErr
		}
		ec.stopConnection(conn, err)
		return n, fmt.Errorf("error writing bytes: %w", err)
	}
	if n != len(data) {
		ec.stopConnection(conn, io.ErrShortWrite)
		return n, io.ErrShortWrite
	}
	return n, nil
}

// Receive returns the next unsolicited complete Glow payload. Request
// responses are routed directly to their requesting method.
func (ec *EmberClient) Receive() ([]byte, error) {
	return ec.ReceiveContext(context.Background())
}

func (ec *EmberClient) ReceiveContext(ctx context.Context) ([]byte, error) {
	message, err := ec.nextNotification(ctx, true)
	if err != nil {
		return nil, err
	}
	return message.raw, nil
}

// ReceiveRootContext returns the next unsolicited decoded Glow root.
func (ec *EmberClient) ReceiveRootContext(ctx context.Context) (ember.RootMessage, error) {
	message, err := ec.nextNotification(ctx, true)
	if err != nil {
		return ember.RootMessage{}, err
	}
	if message.rootErr != nil {
		return ember.RootMessage{}, fmt.Errorf("failed to decode Glow root: %w", message.rootErr)
	}
	return message.root, nil
}

func (ec *EmberClient) ReceiveRoot() (ember.RootMessage, error) {
	return ec.ReceiveRootContext(context.Background())
}

func (ec *EmberClient) nextNotification(ctx context.Context, useTimeout bool) (receivedMessage, error) {
	if err := ec.ensurePump(); err != nil {
		return receivedMessage{}, err
	}
	ec.stateMu.Lock()
	notifications, timeout := ec.notifications, ec.timeout
	ec.stateMu.Unlock()
	if useTimeout {
		var cancel context.CancelFunc
		ctx, cancel = contextWithTimeout(ctx, timeout)
		defer cancel()
	}
	select {
	case message, ok := <-notifications:
		if !ok {
			ec.stateMu.Lock()
			err := ec.pumpErr
			ec.stateMu.Unlock()
			if err == nil {
				err = io.EOF
			}
			return receivedMessage{}, err
		}
		return message, nil
	case <-ctx.Done():
		return receivedMessage{}, ctx.Err()
	}
}

// Serve processes unsolicited messages. It is safe to call request methods
// concurrently; the read pump remains the sole connection reader.
func (ec *EmberClient) Serve(ctx context.Context, handler func(ember.RootMessage) error) error {
	if handler == nil {
		return errors.New("nil Glow message handler")
	}
	for {
		message, err := ec.nextNotification(ctx, false)
		if err != nil {
			return err
		}
		if message.rootErr != nil {
			return fmt.Errorf("failed to decode Glow root: %w", message.rootErr)
		}
		if err := handler(message.root); err != nil {
			return err
		}
	}
}

// LatestElement returns the most recently received top-level qualified element
// for path. Returned elements are immutable snapshots owned by the decoder.
func (ec *EmberClient) LatestElement(path string) (*ember.Element, bool) {
	ec.stateMu.Lock()
	defer ec.stateMu.Unlock()
	element, ok := ec.latestElements[path]
	return element, ok
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

func (ec *EmberClient) GetElementCollectionGlow250(emberType ember.ElementType, emberPath string) (ember.ElementCollection, error) {
	return ec.GetElementCollectionGlow250Context(context.Background(), emberType, emberPath)
}

func (ec *EmberClient) GetElementCollectionGlow250Context(ctx context.Context, emberType ember.ElementType, emberPath string) (ember.ElementCollection, error) {
	return ec.getElementCollectionContext(ctx, emberType, emberPath, true)
}

func (ec *EmberClient) getElementCollectionContext(ctx context.Context, emberType ember.ElementType, emberPath string, glow250 bool) (ember.ElementCollection, error) {
	if !ec.IsConnected() {
		return nil, errors.New("not connected")
	}
	glow, err := ember.EncodeGetDirectory(emberType, emberPath, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to get Ember request. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}
	message, err := ec.doRequest(ctx, "get_directory", emberPath, s101.Encode(glow, s101.FirstMultiPacket), directoryMatcher(emberType, emberPath))
	if err != nil {
		return nil, fmt.Errorf("failed to get Ember answer. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}
	collection := ember.NewElementCollection()
	if glow250 {
		err = collection.PopulateGlow250(asn1.NewDecoder(message.raw))
	} else {
		err = collection.Populate(asn1.NewDecoder(message.raw))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to process Ember answer. Type: %v, Path: %v: %w", emberType, emberPath, err)
	}
	return collection, nil
}

func directoryMatcher(elementType ember.ElementType, path string) func(ember.RootMessage) bool {
	isNode := elementType == ember.NodeElement || elementType == ember.QualifiedNodeElement ||
		string(elementType) == asn1.NodeType || string(elementType) == asn1.QualifiedNodeType
	return func(message ember.RootMessage) bool {
		if message.Elements == nil {
			return false
		}
		if len(message.Elements) == 0 {
			return true
		}
		for key, element := range message.Elements {
			if path == "" || isNode {
				if parentPath(key.Path) == path {
					continue
				}
				if key.Path == path && len(element.Children) > 0 {
					continue
				}
				return false
			}
			if key.Path != path {
				return false
			}
		}
		return true
	}
}

func parentPath(path string) string {
	index := strings.LastIndexByte(path, '.')
	if index < 0 {
		return ""
	}
	return path[:index]
}

func (ec *EmberClient) SetParameter(ctx context.Context, path string, value any) (ember.RootMessage, error) {
	glow, err := ember.EncodeSetParameter(path, value)
	if err != nil {
		return ember.RootMessage{}, err
	}
	message, err := ec.doRequest(ctx, "set_parameter", path, s101.Encode(glow, s101.SinglePacket), elementPathMatcher(path, ember.ParameterElement, ember.QualifiedParameterElement))
	return message.root, err
}

func (ec *EmberClient) SetMatrixConnections(ctx context.Context, path string, connections []ember.MatrixConnection) (ember.RootMessage, error) {
	glow, err := ember.EncodeMatrixConnections(path, connections)
	if err != nil {
		return ember.RootMessage{}, err
	}
	message, err := ec.doRequest(ctx, "set_matrix", path, s101.Encode(glow, s101.SinglePacket), elementPathMatcher(path, ember.MatrixElement, ember.QualifiedMatrixElement))
	return message.root, err
}

func elementPathMatcher(path string, types ...ember.ElementType) func(ember.RootMessage) bool {
	return func(message ember.RootMessage) bool {
		for key, element := range message.Elements {
			if key.Path != path {
				continue
			}
			for _, elementType := range types {
				if element.ElementType == elementType {
					return true
				}
			}
		}
		return false
	}
}

func (ec *EmberClient) Invoke(ctx context.Context, path string, id int64, arguments ...any) (*ember.InvocationResult, error) {
	glow, err := ember.EncodeInvocation(path, id, arguments)
	if err != nil {
		return nil, err
	}
	match := func(message ember.RootMessage) bool {
		return message.InvocationResult != nil && (id == 0 || message.InvocationResult.ID == id)
	}
	message, err := ec.doRequest(ctx, "invoke", path, s101.Encode(glow, s101.SinglePacket), match)
	if err != nil {
		return nil, err
	}
	return message.root.InvocationResult, nil
}

func (ec *EmberClient) Subscribe(ctx context.Context, elementType ember.ElementType, path string, subscribe bool) error {
	glow, err := ember.EncodeSubscription(elementType, path, subscribe)
	if err != nil {
		return err
	}
	ec.requestMu.Lock()
	defer ec.requestMu.Unlock()
	if err := ec.ensurePump(); err != nil {
		return err
	}
	_, err = ec.WriteContext(ctx, s101.Encode(glow, s101.SinglePacket))
	return err
}

// KeepAlive sends a request; the pump routes its frame-level response without
// disturbing Glow messages.
func (ec *EmberClient) KeepAlive(ctx context.Context) error {
	ec.requestMu.Lock()
	defer ec.requestMu.Unlock()
	ctx, cancel := ec.operationContext(ctx)
	defer cancel()
	result := make(chan error, 1)
	ec.stateMu.Lock()
	if ec.conn == nil {
		ec.stateMu.Unlock()
		return errors.New("not connected")
	}
	ec.keepAliveWaiter = result
	if ec.pumpDone == nil {
		ec.startPumpLocked(ec.conn)
	}
	ec.stateMu.Unlock()
	defer ec.clearKeepAliveWaiter(result)
	request, err := s101.EncodeKeepAlive(s101.CommandKeepAliveRequest)
	if err != nil {
		return err
	}
	if _, err := ec.WriteContext(ctx, request); err != nil {
		return err
	}
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (ec *EmberClient) clearKeepAliveWaiter(waiter chan error) {
	ec.stateMu.Lock()
	if ec.keepAliveWaiter == waiter {
		ec.keepAliveWaiter = nil
	}
	ec.stateMu.Unlock()
}

func (ec *EmberClient) doRequest(ctx context.Context, request, path string, wire []byte, match func(ember.RootMessage) bool) (receivedMessage, error) {
	ec.requestMu.Lock()
	defer ec.requestMu.Unlock()
	ctx, cancel := ec.operationContext(ctx)
	defer cancel()
	waiter := &responseWaiter{request: request, path: path, started: time.Now(), match: match, result: make(chan requestResult, 1)}
	ec.stateMu.Lock()
	if ec.conn == nil {
		ec.stateMu.Unlock()
		return receivedMessage{}, errors.New("not connected")
	}
	ec.waiter = waiter
	if ec.pumpDone == nil {
		ec.startPumpLocked(ec.conn)
	}
	ec.stateMu.Unlock()
	defer ec.clearWaiter(waiter)
	if _, err := ec.WriteContext(ctx, wire); err != nil {
		return receivedMessage{}, err
	}
	select {
	case result := <-waiter.result:
		return result.message, result.err
	case <-ctx.Done():
		return receivedMessage{}, ctx.Err()
	}
}

func (ec *EmberClient) clearWaiter(waiter *responseWaiter) {
	ec.stateMu.Lock()
	if ec.waiter == waiter {
		ec.waiter = nil
	}
	ec.stateMu.Unlock()
}

func (ec *EmberClient) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	ec.stateMu.Lock()
	timeout := ec.timeout
	ec.stateMu.Unlock()
	return contextWithTimeout(ctx, timeout)
}

func contextWithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

func (ec *EmberClient) readPump(ctx context.Context, conn net.Conn, notifications chan receivedMessage, done chan struct{}) {
	defer close(done)
	defer close(notifications)
	decoder := s101.NewStreamDecoder()
	var payload []byte
	multipart := false
	buffer := make([]byte, 4096)
	for {
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			ec.finishPump(conn, fmt.Errorf("failed to clear connection read deadline: %w", err))
			return
		}
		n, err := conn.Read(buffer)
		if err != nil {
			if ctx.Err() != nil {
				ec.finishPump(conn, ctx.Err())
			} else {
				ec.finishPump(conn, fmt.Errorf("failed to read from connection: %w", err))
			}
			return
		}
		frames, err := decoder.Push(buffer[:n])
		if err != nil {
			ec.finishPump(conn, fmt.Errorf("failed to decode S101 stream: %w", err))
			return
		}
		for _, frame := range frames {
			switch frame.Command {
			case s101.CommandKeepAliveRequest:
				if err := ec.answerKeepAlive(ctx, frame); err != nil {
					ec.finishPump(conn, err)
					return
				}
				continue
			case s101.CommandKeepAliveResponse:
				ec.routeKeepAlive(nil)
				continue
			case s101.CommandEmber:
			default:
				ec.finishPump(conn, fmt.Errorf("unsupported S101 command: %d", frame.Command))
				return
			}

			complete := false
			switch frame.Flags {
			case s101.FirstMultiPacket:
				payload = append(payload[:0], frame.Payload...)
				multipart = true
			case s101.BodyMultiPacket:
				if !multipart {
					ec.finishPump(conn, errors.New("S101 body packet without first packet"))
					return
				}
				payload = append(payload, frame.Payload...)
			case s101.LastMultiPacket:
				if !multipart {
					ec.finishPump(conn, errors.New("S101 last packet without first packet"))
					return
				}
				payload = append(payload, frame.Payload...)
				complete = true
				multipart = false
			case s101.SinglePacket:
				if multipart {
					ec.finishPump(conn, errors.New("single packet in the middle of a multipart message"))
					return
				}
				payload = append(payload[:0], frame.Payload...)
				complete = true
			default:
				ec.finishPump(conn, fmt.Errorf("invalid S101 packet flag: %x", frame.Flags))
				return
			}
			if complete {
				raw := append([]byte(nil), payload...)
				root, rootErr := ember.DecodeRoot(raw)
				ec.routeMessage(receivedMessage{raw: raw, root: root, rootErr: rootErr}, notifications)
				payload = payload[:0]
			}
		}
	}
}

func (ec *EmberClient) answerKeepAlive(ctx context.Context, frame s101.Frame) error {
	response, err := s101.EncodeFrame(s101.Frame{Framing: frame.Framing, Slot: frame.Slot, Command: s101.CommandKeepAliveResponse})
	if err != nil {
		return err
	}
	if _, err := ec.WriteContext(ctx, response); err != nil {
		return fmt.Errorf("failed to answer S101 keep-alive: %w", err)
	}
	return nil
}

func (ec *EmberClient) routeMessage(message receivedMessage, notifications chan receivedMessage) {
	var diagnostic *Diagnostic
	ec.stateMu.Lock()
	if message.rootErr == nil {
		for key, element := range message.root.Elements {
			ec.latestElements[key.Path] = element
		}
	}
	waiter := ec.waiter
	if waiter != nil && message.rootErr == nil && waiter.match(message.root) {
		ec.waiter = nil
		waiter.result <- requestResult{message: message}
		if waiter.skipped > 0 {
			diagnostic = &Diagnostic{Kind: DiagnosticRequestBacklog, Request: waiter.request, Path: waiter.path, SkippedRoots: waiter.skipped, Duration: time.Since(waiter.started)}
		}
		ec.stateMu.Unlock()
		if diagnostic != nil {
			ec.emit(*diagnostic)
		}
		return
	}
	if waiter != nil {
		waiter.skipped++
	}
	ec.stateMu.Unlock()

	select {
	case notifications <- message:
		return
	default:
	}
	select {
	case <-notifications:
	default:
	}
	ec.stateMu.Lock()
	ec.dropped++
	dropped := ec.dropped
	ec.stateMu.Unlock()
	select {
	case notifications <- message:
	default:
	}
	ec.emit(Diagnostic{Kind: DiagnosticNotificationOverflow, DroppedNotifications: dropped})
}

func (ec *EmberClient) routeKeepAlive(err error) {
	ec.stateMu.Lock()
	waiter := ec.keepAliveWaiter
	if waiter != nil {
		ec.keepAliveWaiter = nil
	}
	ec.stateMu.Unlock()
	if waiter != nil {
		waiter <- err
	}
}

func (ec *EmberClient) finishPump(conn net.Conn, err error) {
	ec.stateMu.Lock()
	if ec.conn == conn {
		ec.conn = nil
		ec.pumpErr = err
		ec.failWaitersLocked(err)
	}
	ec.stateMu.Unlock()
	_ = conn.Close()
	if !errors.Is(err, context.Canceled) {
		ec.emit(Diagnostic{Kind: DiagnosticReadPumpError, Err: err})
	}
}

func (ec *EmberClient) failWaitersLocked(err error) {
	if ec.waiter != nil {
		ec.waiter.result <- requestResult{err: err}
		ec.waiter = nil
	}
	if ec.keepAliveWaiter != nil {
		ec.keepAliveWaiter <- err
		ec.keepAliveWaiter = nil
	}
}

func (ec *EmberClient) stopConnection(conn net.Conn, cause error) {
	ec.stateMu.Lock()
	if ec.conn != conn {
		ec.stateMu.Unlock()
		return
	}
	cancel := ec.pumpCancel
	ec.conn = nil
	ec.pumpErr = cause
	ec.failWaitersLocked(cause)
	ec.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	_ = conn.Close()
}

func (ec *EmberClient) connection() (net.Conn, time.Duration) {
	ec.stateMu.Lock()
	defer ec.stateMu.Unlock()
	return ec.conn, ec.timeout
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
