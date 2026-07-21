package s101

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamDecoderPreservesCompleteAndPartialFrames(t *testing.T) {
	t.Parallel()

	first := Encode([]byte{1}, SinglePacket)
	second := Encode([]byte{2, 3}, SinglePacket)
	decoder := NewStreamDecoder()
	cut := len(second) / 2
	frames, err := decoder.Push(append(append([]byte(nil), first...), second[:cut]...))
	require.NoError(t, err)
	require.Len(t, frames, 1)
	assert.Equal(t, []byte{1}, frames[0].Payload)

	frames, err = decoder.Push(second[cut:])
	require.NoError(t, err)
	require.Len(t, frames, 1)
	assert.Equal(t, []byte{2, 3}, frames[0].Payload)
}

func TestKeepAliveRoundTrip(t *testing.T) {
	t.Parallel()

	encoded, err := EncodeKeepAlive(CommandKeepAliveRequest)
	require.NoError(t, err)
	frame, err := ParseFrame(encoded)
	require.NoError(t, err)
	assert.Equal(t, byte(CommandKeepAliveRequest), frame.Command)
	assert.Equal(t, FramingEscaped, frame.Framing)
}

func TestNonEscapingFrameRoundTrip(t *testing.T) {
	t.Parallel()

	expected := Frame{Framing: FramingUnescaped, Command: CommandEmber, Flags: SinglePacket, Payload: []byte{0xfe, 0xff, 1}}
	encoded, err := EncodeFrame(expected)
	require.NoError(t, err)
	actual, err := ParseFrame(encoded)
	require.NoError(t, err)
	assert.Equal(t, FramingUnescaped, actual.Framing)
	assert.Equal(t, byte(50), actual.GlowMinor)
	assert.Equal(t, expected.Payload, actual.Payload)
}

func TestEscapedFrameCRCWithLiteralEscapeByte(t *testing.T) {
	t.Parallel()

	expected := []byte{0xfd, 0x01, 0xff}
	encoded := Encode(expected, SinglePacket)
	frame, err := ParseFrame(encoded)
	require.NoError(t, err)
	assert.Equal(t, expected, frame.Payload)
}

func TestEscapedFrameEscapesReservedCRCByte(t *testing.T) {
	t.Parallel()

	var payload []byte
	var reserved byte
	for value := 0; value <= 0xffff; value++ {
		candidate := []byte{byte(value >> 8), byte(value)}
		body, err := frameBody(Frame{Command: CommandEmber, Flags: SinglePacket, Payload: candidate})
		require.NoError(t, err)
		crc := rawChecksum(body)
		if crc[0] >= bofne || crc[1] >= bofne {
			payload = candidate
			if crc[0] >= bofne {
				reserved = crc[0]
			} else {
				reserved = crc[1]
			}
			break
		}
	}
	require.NotNil(t, payload)

	encoded := Encode(payload, SinglePacket)
	assert.True(t, bytes.Contains(encoded, []byte{ce, reserved ^ xorce}))
	frame, err := ParseFrame(encoded)
	require.NoError(t, err)
	assert.Equal(t, payload, frame.Payload)
}
