/*
** Copyright (C) 2001-2024 Zabbix SIA
**
** This program is free software: you can redistribute it and/or modify it under the terms of
** the GNU Affero General Public License as published by the Free Software Foundation, version 3.
**
** This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY;
** without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
** See the GNU Affero General Public License for more details.
**
** You should have received a copy of the GNU Affero General Public License along with this program.
** If not, see <https://www.gnu.org/licenses/>.
**/

package s101

import (
	"bytes"
	"errors"
	"fmt"
)

// Framing identifies the S101 framing variant used by a frame.
type Framing uint8

const (
	// FramingEscaped is the mandatory, CRC-protected S101 framing variant.
	FramingEscaped Framing = 1
	// FramingUnescaped is the optional length-prefixed Glow 2.50 framing variant.
	FramingUnescaped Framing = 2
)

// Frame is a decoded S101 message.
type Frame struct {
	Framing     Framing
	Slot        byte
	MessageType byte
	Command     byte
	Version     byte
	Flags       byte
	DTD         byte
	GlowMajor   byte
	GlowMinor   byte
	Payload     []byte
}

// StreamDecoder separates S101 frames independently of TCP read boundaries.
type StreamDecoder struct {
	buffer []byte
}

// NewStreamDecoder creates an empty S101 stream decoder.
func NewStreamDecoder() *StreamDecoder { return &StreamDecoder{} }

// Push appends bytes and returns every complete frame now available.
func (d *StreamDecoder) Push(data []byte) ([]Frame, error) {
	d.buffer = append(d.buffer, data...)
	var frames []Frame
	for {
		start := indexFrameStart(d.buffer)
		if start < 0 {
			d.buffer = d.buffer[:0]
			return frames, nil
		}
		if start > 0 {
			d.buffer = d.buffer[start:]
		}

		var raw []byte
		switch d.buffer[0] {
		case bof:
			end := bytes.IndexByte(d.buffer[1:], eof)
			if end < 0 {
				return frames, nil
			}
			end += 2
			raw = append([]byte(nil), d.buffer[:end]...)
			d.buffer = d.buffer[end:]
		case bofne:
			if len(d.buffer) < 2 {
				return frames, nil
			}
			lengthBytes := int(d.buffer[1] & 0x07)
			if d.buffer[1]&0xf8 != 0 {
				return nil, errors.New("reserved non-escaping S101 flag is set")
			}
			if len(d.buffer) < 2+lengthBytes {
				return frames, nil
			}
			payloadLength := 0
			for _, b := range d.buffer[2 : 2+lengthBytes] {
				if payloadLength > int(^uint(0)>>1)>>8 {
					return nil, errors.New("non-escaping S101 payload length overflows int")
				}
				payloadLength = payloadLength<<8 | int(b)
			}
			total := 2 + lengthBytes + payloadLength
			if len(d.buffer) < total {
				return frames, nil
			}
			raw = append([]byte(nil), d.buffer[:total]...)
			d.buffer = d.buffer[total:]
		}

		frame, err := ParseFrame(raw)
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	}
}

func indexFrameStart(data []byte) int {
	escaped := bytes.IndexByte(data, bof)
	unescaped := bytes.IndexByte(data, bofne)
	if escaped < 0 {
		return unescaped
	}
	if unescaped < 0 || escaped < unescaped {
		return escaped
	}
	return unescaped
}

// Encode creates a 101 packet from the message adding all the required S101 bytes based on the S101 protocol, if
// package type is multi packet message, adds an empty packet to the end as require by the protocol.
func Encode(message []byte, packetType uint8) []uint8 {
	out := createS101(message, packetType)
	if packetType == FirstMultiPacket {
		out = append(out, createS101([]byte{}, LastMultiPacket)...)
	}

	return out
}

// EncodeKeepAlive creates an escaped keep-alive request or response frame.
func EncodeKeepAlive(command byte) ([]byte, error) {
	if command != CommandKeepAliveRequest && command != CommandKeepAliveResponse {
		return nil, fmt.Errorf("invalid keep-alive command: %d", command)
	}
	return createEscapedFrame([]byte{slot, messageType, command, version}), nil
}

// EncodeFrame encodes a structured frame using either S101 framing variant.
func EncodeFrame(frame Frame) ([]byte, error) {
	body, err := frameBody(frame)
	if err != nil {
		return nil, err
	}
	if frame.Framing == FramingUnescaped {
		length := len(body)
		if uint64(length) > uint64(^uint32(0)) {
			return nil, errors.New("non-escaping S101 frame exceeds uint32 length")
		}
		out := []byte{bofne, 4, byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)}
		return append(out, body...), nil
	}
	return createEscapedFrame(body), nil
}

func frameBody(frame Frame) ([]byte, error) {
	message := frame.MessageType
	if message == 0 {
		message = messageType
	}
	ver := frame.Version
	if ver == 0 {
		ver = version
	}
	body := []byte{frame.Slot, message, frame.Command, ver}
	if frame.Command != CommandEmber {
		if frame.Command != CommandKeepAliveRequest && frame.Command != CommandKeepAliveResponse {
			return nil, fmt.Errorf("unsupported S101 command: %d", frame.Command)
		}
		return body, nil
	}
	major, minor := frame.GlowMajor, frame.GlowMinor
	if major == 0 {
		major = majorVersion
	}
	if minor == 0 {
		minor = minorVersion
	}
	dtd := frame.DTD
	if dtd == 0 {
		dtd = dtdType
	}
	body = append(body, frame.Flags, dtd, appBytes, minor, major)
	return append(body, frame.Payload...), nil
}

// ParseFrame validates and decodes one complete S101 frame.
func ParseFrame(raw []byte) (Frame, error) {
	var (
		body    []byte
		framing Framing
	)
	if len(raw) == 0 {
		return Frame{}, errors.New("empty S101 frame")
	}
	switch raw[0] {
	case bof:
		decoded, err := verifyAndStripCRC(raw)
		if err != nil {
			return Frame{}, err
		}
		body = decoded
		framing = FramingEscaped
	case bofne:
		if len(raw) < 2 {
			return Frame{}, errors.New("incomplete non-escaping S101 header")
		}
		lengthBytes := int(raw[1] & 0x07)
		if raw[1]&0xf8 != 0 || len(raw) < 2+lengthBytes {
			return Frame{}, errors.New("invalid non-escaping S101 header")
		}
		length := 0
		for _, b := range raw[2 : 2+lengthBytes] {
			if length > int(^uint(0)>>1)>>8 {
				return Frame{}, errors.New("non-escaping S101 payload length overflows int")
			}
			length = length<<8 | int(b)
		}
		if len(raw) != 2+lengthBytes+length {
			return Frame{}, errors.New("non-escaping S101 payload length mismatch")
		}
		body = raw[2+lengthBytes:]
		framing = FramingUnescaped
	default:
		return Frame{}, errors.New("unknown S101 framing")
	}

	if len(body) < 4 {
		return Frame{}, errors.New("S101 message header is too short")
	}
	frame := Frame{Framing: framing, Slot: body[0], MessageType: body[1], Command: body[2], Version: body[3]}
	if frame.MessageType != messageType {
		return Frame{}, fmt.Errorf("unsupported S101 message type: %x", frame.MessageType)
	}
	if frame.Version != version {
		return Frame{}, fmt.Errorf("unsupported S101 version: %d", frame.Version)
	}
	if frame.Command != CommandEmber {
		if frame.Command != CommandKeepAliveRequest && frame.Command != CommandKeepAliveResponse {
			return Frame{}, fmt.Errorf("unsupported S101 command: %d", frame.Command)
		}
		return frame, nil
	}
	if len(body) < 7 {
		return Frame{}, errors.New("Ember S101 header is too short")
	}
	applicationBytes := int(body[6])
	if applicationBytes < 2 || len(body) < 7+applicationBytes {
		return Frame{}, errors.New("invalid S101 application header length")
	}
	frame.Flags = body[4]
	frame.DTD = body[5]
	frame.GlowMinor = body[7]
	frame.GlowMajor = body[8]
	frame.Payload = append([]byte(nil), body[7+applicationBytes:]...)
	return frame, nil
}

func unescape(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		if data[i] != ce {
			out = append(out, data[i])
			continue
		}
		if i+1 >= len(data) {
			return nil, errors.New("truncated S101 escape")
		}
		i++
		out = append(out, data[i]^xorce)
	}
	return out, nil
}

// GetS101s returns all s101 data packets from message, if the message contains an incomplete packet it will return the
// raw data in the second response value.
func GetS101s(message []byte) ([][]byte, []byte, error) {
	if len(message) == 0 {
		return nil, nil, errors.New("no data")
	}

	s101s, incompleteData := getS101s(message)

	return s101s, incompleteData, nil
}

// Decode removes all the S101 addons from the packet returning only glow data.
func Decode(s101s [][]byte) ([]byte, byte, error) {
	var (
		out            []byte
		lastPacketType byte
	)

	for i, s101 := range s101s {
		frame, err := ParseFrame(s101)
		if err != nil {
			return nil, 0, err
		}
		if frame.Command != CommandEmber {
			return nil, 0, fmt.Errorf("S101 frame does not contain Ember data: command %d", frame.Command)
		}
		if i == len(s101s)-1 {
			lastPacketType = frame.Flags
		}
		out = append(out, frame.Payload...)
	}

	return out, lastPacketType, nil
}

func verifyAndStripCRC(s101 []byte) ([]byte, error) {
	// BOF + the four mandatory S101 header bytes + CRC16 + EOF.
	if len(s101) < 8 {
		return nil, fmt.Errorf("malformed s101 packet, malformed s101 data: %x", s101)
	}
	if s101[0] != bof || s101[len(s101)-1] != eof {
		return nil, fmt.Errorf("malformed s101 packet, missing frame boundary: %x", s101)
	}

	decoded, err := unescape(s101[1 : len(s101)-1])
	if err != nil {
		return nil, err
	}
	if len(decoded) < 6 {
		return nil, errors.New("S101 frame is too short for its checksum")
	}
	body := decoded[:len(decoded)-2]
	if bytes.Equal(rawChecksum(body), decoded[len(decoded)-2:]) {
		return body, nil
	}

	return nil, fmt.Errorf("invalid s101 checksum: %x", s101)
}

// getS101s reads the last entry in the byte array start starts with BOF byte and ends with EOF byte.
// if data is incomplete, returns it as second parameter.
func getS101s(in []uint8) ([][]uint8, []uint8) {
	var (
		startFound bool
	)

	r := bytes.NewBuffer(in)

	var out [][]uint8

	var single []uint8

	for {
		b, err := r.ReadByte()
		if err != nil {
			if startFound {
				// no closing byte found assuming packet is sent in multiple writes, we return raw data.
				return out, single
			}

			return out, nil
		}

		if b == bof {
			startFound = true

			// a valid glow packet should not have multiple FE without FF, so we are interested in reading only the
			// last valid glow data, incase there is some left over invalid data at the beginning of the frame.
			single = []uint8{}

			single = append(single, b)

			continue
		}

		if startFound {
			single = append(single, b)

			if b == eof {
				startFound = false
				out = append(out, single)

				single = []uint8{}
			}
		}
	}
}

// createS101 creates a S101 packet from the provided payload and packet type.
func createS101(payload []byte, pType uint8) []byte {
	s101Info := []byte{slot, messageType, commandType, version, pType, dtdType, appBytes, minorVersion, majorVersion}
	return createEscapedFrame(append(s101Info, payload...))
}

func createEscapedFrame(body []byte) []byte {
	encoded := append(append([]byte(nil), body...), rawChecksum(body)...)
	escaped := escapeBytesAboveBOFNE(encoded)
	s101 := make([]byte, 0, len(escaped)+2)
	s101 = append(s101, bof)
	s101 = append(s101, escaped...)
	s101 = append(s101, eof)

	return s101
}

// escapeBytesAboveBOFNE parses the message as based on Glow protocol all the bytes with bigger value then 0xf8 must
// preceded with and 0xfd byte and XORed with 0x20 byte.
func escapeBytesAboveBOFNE(message []byte) []byte {
	//nolint:prealloc
	var out []byte

	for _, b := range message {
		if b >= bofne {
			out = append(out, ce, xorce^b)

			continue
		}

		out = append(out, b)
	}

	return out
}

// getCRC prepares and returns CRC for S101 packet based on the payload, crc is generated based on the S101 protocol
// requirements.
func getCRC(data []byte) []uint8 {
	decoded, err := unescape(data)
	if err != nil {
		decoded = data[:len(data)-1]
	}
	return checksum(decoded)
}

func checksum(data []byte) []byte {
	return parseCRC(rawChecksum(data))
}

func rawChecksum(data []byte) []byte {
	crc := uint16(eof16)
	for _, b := range data {
		crc = computeCRCByte(crc, b)
	}
	crc = ^crc
	return []byte{byte(crc), byte(crc >> checkSumSecondDeviation)}
}

// parseCRC bytes above 0xf8 must be preceded with and 0xfd byte and XORed with 0x20 byte.
func parseCRC(in []uint8) []uint8 {
	//nolint:prealloc
	var out []uint8

	for _, v := range in {
		if v < bofne {
			out = append(out, v)

			continue
		}

		out = append(out, ce, v^xorce)
	}

	return out
}

// computeCRCByte creates crc double byte value bases on the S101 crc table against current crc byte using the
// provided byte.
func computeCRCByte(crc uint16, b uint8) uint16 {
	return ((crc >> 8) ^ crcTable[(crc^uint16(b))&0xFF]) & 0xFFFF
}
