package emberclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/johannes-kuhfuss/emberplus/asn1"
	"github.com/johannes-kuhfuss/emberplus/ember"
	"github.com/johannes-kuhfuss/emberplus/s101"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeConn struct {
	reads       [][]byte
	readErr     error
	writeErr    error
	shortWrite  bool
	deadlineErr error
	written     bytes.Buffer
	deadlines   []time.Time
	closed      bool
}

func (fc *fakeConn) Read(b []byte) (int, error) {
	if len(fc.reads) == 0 {
		if fc.readErr != nil {
			return 0, fc.readErr
		}

		return 0, io.EOF
	}

	next := fc.reads[0]
	fc.reads = fc.reads[1:]

	return copy(b, next), nil
}

func (fc *fakeConn) Write(b []byte) (int, error) {
	if fc.writeErr != nil {
		return 0, fc.writeErr
	}

	if fc.shortWrite {
		n := len(b) / 2
		fc.written.Write(b[:n])

		return n, nil
	}

	return fc.written.Write(b)
}

func (fc *fakeConn) Close() error {
	fc.closed = true

	return nil
}

func (fc *fakeConn) LocalAddr() net.Addr {
	return &net.TCPAddr{}
}

func (fc *fakeConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{}
}

func (fc *fakeConn) SetDeadline(t time.Time) error {
	if fc.deadlineErr != nil {
		return fc.deadlineErr
	}

	fc.deadlines = append(fc.deadlines, t)

	return nil
}

func (fc *fakeConn) SetReadDeadline(time.Time) error {
	return nil
}

func (fc *fakeConn) SetWriteDeadline(time.Time) error {
	if fc.deadlineErr != nil {
		return fc.deadlineErr
	}

	fc.deadlines = append(fc.deadlines, time.Now())

	return nil
}

func startTCPServer(t *testing.T) (string, int, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		<-done
	}()

	cleanup := func() {
		close(done)
		require.NoError(t, listener.Close())
	}

	addr := listener.Addr().(*net.TCPAddr)

	return "127.0.0.1", addr.Port, cleanup
}

func validRootGlow() []byte {
	return []byte{
		0x60, 0x34, 0x6B, 0x32, 0xA0, 0x30, 0x63, 0x2E, 0xA0, 0x03, 0x02, 0x01, 0x01, 0xA1, 0x27, 0x31,
		0x25, 0xA0, 0x16, 0x0C, 0x14, 0x52, 0x33, 0x4C, 0x41, 0x59, 0x56, 0x69, 0x72, 0x74, 0x75, 0x61,
		0x6C, 0x50, 0x61, 0x74, 0x63, 0x68, 0x42, 0x61, 0x79, 0xA1, 0x02, 0x0C, 0x00, 0xA4, 0x02, 0x0C,
		0x00, 0xA3, 0x03, 0x01, 0x01, 0xFF,
	}
}

func parameterRootGlow(t *testing.T, withValue bool) []byte {
	t.Helper()

	path, err := asn1.MarshalRelativeOID([]int{1})
	require.NoError(t, err)
	pathField, err := asn1.MarshalExplicit(0, path)
	require.NoError(t, err)
	typeField, err := asn1.MarshalExplicit(13, asn1.MarshalInteger(1))
	require.NoError(t, err)
	fields := [][]byte{typeField}
	if withValue {
		valueField, marshalErr := asn1.MarshalExplicit(2, asn1.MarshalInteger(7))
		require.NoError(t, marshalErr)
		fields = append(fields, valueField)
	}
	contents, err := asn1.MarshalContainer(asn1.ClassUniversal, 17, fields...)
	require.NoError(t, err)
	contentsField, err := asn1.MarshalExplicit(1, contents)
	require.NoError(t, err)
	parameter, err := asn1.MarshalContainer(asn1.ClassApplication, 9, pathField, contentsField)
	require.NoError(t, err)
	wrapper, err := asn1.MarshalExplicit(0, parameter)
	require.NoError(t, err)
	collection, err := asn1.MarshalContainer(asn1.ClassApplication, 11, wrapper)
	require.NoError(t, err)
	root, err := asn1.MarshalContainer(asn1.ClassApplication, 0, collection)
	require.NoError(t, err)
	return root
}

func waitForReader(t *testing.T, ec *EmberClient) {
	t.Helper()
	require.Eventually(t, func() bool {
		if !ec.readerMu.TryLock() {
			return true
		}
		ec.readerMu.Unlock()
		return false
	}, time.Second, 5*time.Millisecond)
}

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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	addr := listener.Addr().(*net.TCPAddr)
	assert.NoError(t, listener.Close())

	ec, _ := NewEmberClient("127.0.0.1", addr.Port)
	err = ec.Connect()
	assert.NotNil(t, err)
	assert.EqualValues(t, false, ec.IsConnected())
}

func TestConnectCanConnectReturnsNoError(t *testing.T) {
	host, port, cleanup := startTCPServer(t)
	defer cleanup()

	ec, _ := NewEmberClient(host, port)
	err := ec.Connect()
	assert.Nil(t, err)
	assert.EqualValues(t, true, ec.IsConnected())
	ec.Disconnect()
}

func TestConnectAlreadyConnectedReturnsError(t *testing.T) {
	host, port, cleanup := startTCPServer(t)
	defer cleanup()

	ec, _ := NewEmberClient(host, port)
	ec.Connect()
	err := ec.Connect()
	assert.NotNil(t, err)
	assert.EqualValues(t, "already connected", err.Error())
	ec.Disconnect()
}

func TestDisconnectNoConnectionReturnsError(t *testing.T) {
	ec, _ := NewEmberClient("127.0.0.1", 9000)
	err := ec.Disconnect()
	assert.NotNil(t, err)
	assert.EqualValues(t, "not connected", err.Error())
}

func TestDisconnectConnectedReturnsNoError(t *testing.T) {
	ec := &EmberClient{conn: &fakeConn{}, timeout: time.Second}

	err := ec.Disconnect()

	assert.Nil(t, err)
	assert.False(t, ec.IsConnected())
}

func TestSetTimeoutAndWrite(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{}
	ec := &EmberClient{conn: conn, timeout: time.Second}

	ec.SetTimeout(2 * time.Second)
	n, err := ec.Write([]byte{1, 2, 3})

	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, []byte{1, 2, 3}, conn.written.Bytes())
	assert.Len(t, conn.deadlines, 1)
}

func TestWriteWithoutConnectionReturnsError(t *testing.T) {
	t.Parallel()

	ec := &EmberClient{}
	n, err := ec.Write([]byte{1})

	assert.Equal(t, 0, n)
	require.EqualError(t, err, "not connected")
}

func TestWriteShortWriteReturnsError(t *testing.T) {
	t.Parallel()

	ec := &EmberClient{conn: &fakeConn{shortWrite: true}, timeout: time.Second}
	n, err := ec.Write([]byte{1, 2, 3, 4})

	assert.Equal(t, 2, n)
	require.ErrorIs(t, err, io.ErrShortWrite)
	assert.False(t, ec.IsConnected())
}

func TestWriteDeadlineErrorReturnsError(t *testing.T) {
	t.Parallel()

	ec := &EmberClient{conn: &fakeConn{deadlineErr: errors.New("boom")}, timeout: time.Second}
	n, err := ec.Write([]byte{1})

	assert.Equal(t, 0, n)
	require.ErrorContains(t, err, "failed to set connection write deadline")
}

func TestReceiveSinglePacketReturnsGlowData(t *testing.T) {
	t.Parallel()

	want := []byte{1, 2, 3}
	ec := &EmberClient{conn: &fakeConn{reads: [][]byte{s101.Encode(want, s101.SinglePacket)}}, timeout: time.Second}

	got, err := ec.Receive()

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestReceiveCombinesIncompleteS101Reads(t *testing.T) {
	t.Parallel()

	want := []byte{1, 2, 3}
	packet := s101.Encode(want, s101.SinglePacket)
	ec := &EmberClient{conn: &fakeConn{reads: [][]byte{packet[:5], packet[5:]}}, timeout: time.Second}

	got, err := ec.Receive()

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestReceiveMultiPacketReturnsCombinedGlowData(t *testing.T) {
	t.Parallel()

	want := []byte{1, 2, 3}
	ec := &EmberClient{conn: &fakeConn{reads: [][]byte{s101.Encode(want, s101.FirstMultiPacket)}}, timeout: time.Second}

	got, err := ec.Receive()

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestReceiveAnswersKeepAliveBeforeEmberData(t *testing.T) {
	t.Parallel()

	keepAlive, err := s101.EncodeKeepAlive(s101.CommandKeepAliveRequest)
	require.NoError(t, err)
	want := []byte{1, 2, 3}
	read := append(keepAlive, s101.Encode(want, s101.SinglePacket)...)
	conn := &fakeConn{reads: [][]byte{read}}
	ec := &EmberClient{conn: conn, timeout: time.Second}

	got, err := ec.Receive()
	require.NoError(t, err)
	assert.Equal(t, want, got)

	decoder := s101.NewStreamDecoder()
	frames, err := decoder.Push(conn.written.Bytes())
	require.NoError(t, err)
	require.Len(t, frames, 1)
	assert.Equal(t, byte(s101.CommandKeepAliveResponse), frames[0].Command)
}

func TestReceiveWithoutConnectionReturnsError(t *testing.T) {
	t.Parallel()

	ec := &EmberClient{}
	got, err := ec.Receive()

	assert.Nil(t, got)
	require.EqualError(t, err, "not connected")
}

func TestReceiveContextCancelledReturnsError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ec := &EmberClient{conn: &fakeConn{}, timeout: time.Second}

	got, err := ec.ReceiveContext(ctx)

	assert.Nil(t, got)
	require.ErrorIs(t, err, context.Canceled)
}

func TestReceiveContextInterruptsBlockedReadWithoutClientTimeout(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	ec := &EmberClient{conn: client, timeout: 0}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	got, err := ec.ReceiveContext(ctx)
	assert.Nil(t, got)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestReceiveReadErrorReturnsError(t *testing.T) {
	t.Parallel()

	ec := &EmberClient{conn: &fakeConn{readErr: errors.New("boom")}, timeout: time.Second}
	got, err := ec.Receive()

	assert.Nil(t, got)
	require.ErrorContains(t, err, "failed to read from connection")
	assert.False(t, ec.IsConnected())
}

func TestServeIgnoresClientTimeoutAndRejectsConcurrentReader(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	ec := &EmberClient{conn: client, timeout: 20 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handled := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- ec.Serve(ctx, func(ember.RootMessage) error {
			close(handled)
			cancel()
			return nil
		})
	}()
	waitForReader(t, ec)

	_, err := ec.GetRootCollection()
	require.ErrorIs(t, err, ErrReceiveActive)

	time.Sleep(60 * time.Millisecond)
	_, err = server.Write(s101.Encode(validRootGlow(), s101.SinglePacket))
	require.NoError(t, err)

	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatal("Serve did not process data after the client timeout")
	}
	require.ErrorIs(t, <-done, context.Canceled)
}

func TestServeReadFailureClearsConnection(t *testing.T) {
	client, server := net.Pipe()
	ec := &EmberClient{conn: client, timeout: time.Second}
	done := make(chan error, 1)
	go func() {
		done <- ec.Serve(context.Background(), func(ember.RootMessage) error { return nil })
	}()
	waitForReader(t, ec)

	require.NoError(t, server.Close())
	require.Error(t, <-done)
	assert.False(t, ec.IsConnected())
}

func TestPublicGetMethodsRequireConnection(t *testing.T) {
	t.Parallel()

	ec := &EmberClient{}

	gotRoot, err := ec.GetRoot()
	assert.Nil(t, gotRoot)
	require.EqualError(t, err, "not connected")

	gotJSON, err := ec.GetByType(asn1.QualifiedNodeType, "")
	assert.Nil(t, gotJSON)
	require.EqualError(t, err, "not connected")

	gotCollection, err := ec.GetRootCollection()
	assert.Nil(t, gotCollection)
	require.EqualError(t, err, "not connected")

	gotTypedCollection, err := ec.GetElementCollection(asn1.QualifiedNodeType, "")
	assert.Nil(t, gotTypedCollection)
	require.EqualError(t, err, "not connected")
}

func TestGetElementCollectionContextInvalidPathReturnsBeforeWrite(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{}
	ec := &EmberClient{conn: conn, timeout: time.Second}

	got, err := ec.GetElementCollectionContext(context.Background(), ember.ElementType(asn1.QualifiedNodeType), "not-a-path")

	assert.Nil(t, got)
	require.ErrorContains(t, err, "failed to parse path")
	assert.Empty(t, conn.written.Bytes())
}

func TestGetElementCollectionContextReturnsPopulatedCollection(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{reads: [][]byte{s101.Encode(validRootGlow(), s101.SinglePacket)}}
	ec := &EmberClient{conn: conn, timeout: time.Second}

	got, err := ec.GetElementCollectionContext(context.Background(), ember.ElementType(asn1.QualifiedNodeType), "")

	require.NoError(t, err)
	assert.NotEmpty(t, conn.written.Bytes())

	el, path, err := got.GetElementByID("R3LAYVirtualPatchBay")
	require.NoError(t, err)
	assert.Equal(t, "1", path)
	assert.Equal(t, "1", el.Path)
	assert.Equal(t, ember.ElementType(asn1.NodeType), el.ElementType)
	assert.True(t, el.IsOnline)

	frames, err := s101.NewStreamDecoder().Push(conn.written.Bytes())
	require.NoError(t, err)
	require.Len(t, frames, 2)
	assert.Equal(t, byte(s101.FirstMultiPacket), frames[0].Flags)
	assert.Equal(t, byte(s101.LastMultiPacket), frames[1].Flags)
}

func TestExistingGetterKeepsLegacyValuesAndGlow250IsExplicit(t *testing.T) {
	legacyConn := &fakeConn{reads: [][]byte{s101.Encode(parameterRootGlow(t, false), s101.SinglePacket)}}
	legacyClient := &EmberClient{conn: legacyConn, timeout: time.Second}

	legacy, err := legacyClient.GetElementCollection(asn1.QualifiedParameterType, "")
	require.NoError(t, err)
	legacyElement, err := legacy.GetElementByPath("1")
	require.NoError(t, err)
	assert.Equal(t, 0, legacyElement.Value)

	glowConn := &fakeConn{reads: [][]byte{s101.Encode(parameterRootGlow(t, false), s101.SinglePacket)}}
	glowClient := &EmberClient{conn: glowConn, timeout: time.Second}
	glow, err := glowClient.GetElementCollectionGlow250(asn1.QualifiedParameterType, "")
	require.NoError(t, err)
	glowElement, err := glow.GetElementByPath("1")
	require.NoError(t, err)
	assert.Nil(t, glowElement.Value)
	assert.False(t, glowElement.HasValue)
}

func TestGetByTypeContextReturnsJSON(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{reads: [][]byte{s101.Encode(validRootGlow(), s101.SinglePacket)}}
	ec := &EmberClient{conn: conn, timeout: time.Second}

	got, err := ec.GetByTypeContext(context.Background(), ember.ElementType(asn1.QualifiedNodeType), "")

	require.NoError(t, err)
	assert.JSONEq(t, `{"1":{"path":"1","element_type":"node","children":null,"identifier":"R3LAYVirtualPatchBay","description":"","is_online":true,"is_root":false}}`, string(got))
}

func TestGetElementCollectionContextReceiveErrorClosesConnection(t *testing.T) {
	t.Parallel()

	conn := &fakeConn{readErr: errors.New("boom")}
	ec := &EmberClient{conn: conn, timeout: time.Second}

	got, err := ec.GetElementCollectionContext(context.Background(), ember.ElementType(asn1.QualifiedNodeType), "")

	assert.Nil(t, got)
	require.ErrorContains(t, err, "failed to get Ember answer")
	assert.True(t, conn.closed)
	assert.False(t, ec.IsConnected())
}
