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

// Encode creates a 101 packet from the message adding all the required S101 bytes based on the S101 protocol, if
// package type is multi packet message, adds an empty packet to the end as require by the protocol.
func Encode(message []byte, packetType uint8) []uint8 {
	out := createS101(message, packetType)
	if packetType == FirstMultiPacket {
		out = append(out, createS101([]byte{}, LastMultiPacket)...)
	}

	return out
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

// Decode removes all the S101 addons from the packet returning only glow data, currently does not check CRC.
func Decode(s101s [][]byte) ([]byte, byte, error) {
	var (
		out            []byte
		lastPacketType byte
	)

	for i, s101 := range s101s {
		if len(s101) < s101LenTilGlow+s101LenAfterGlow {
			return nil, 0, fmt.Errorf("malformed s101 packet, malformed s101 data: %x", s101)
		}

		if i == len(s101s)-1 {
			lastPacketType = s101[5]
		}

		// remove checksum and end of frame byte, this check is done here as not to XOR a checksum byte
		s101 = s101[:len(s101)-s101LenAfterGlow]

		var (
			ceFound bool
			glow    []byte
		)

		for _, b := range s101 {
			if b == ce {
				ceFound = true

				continue
			}

			if ceFound {
				ceFound = false

				glow = append(glow, xorce^b)

				continue
			}

			glow = append(glow, b)
		}

		out = append(out, glow[s101LenTilGlow:]...)
	}

	return out, lastPacketType, nil
}

// getS101s reads the last entry in the byte array start starts with BOF byte and ends with EOF byte.
// if data is incomplete, returns it as second parameter.
func getS101s(in []uint8) ([][]uint8, []uint8) {
	var (
		startFound bool
		endFound   bool
	)

	r := bytes.NewBuffer(in)

	var out [][]uint8

	var single []uint8

	for {
		b, err := r.ReadByte()
		if err != nil {
			if !endFound {
				// no closing byte found assuming packet is sent in multiple writes, we return raw data.
				return nil, single
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
				endFound = true

				out = append(out, single)

				single = []uint8{}
			}
		}
	}
}

// createS101 creates a S101 packet from the provided payload and packet type.
func createS101(payload []byte, pType uint8) []byte {
	escaped := escapeBytesAboveBOFNE(payload)
	s101Info := []byte{slot, messageType, commandType, version, pType, dtdType, appBytes, minorVersion, majorVersion}
	tmp := make([]byte, 0, len(s101Info)+len(payload))

	tmp = append(tmp, s101Info...)
	crc := getCRC(append(tmp, payload...))

	s101 := make([]byte, 0, len(s101Info)+len(escaped)+len(crc)+2)
	s101 = append(s101, bof)
	s101 = append(s101, s101Info...)
	s101 = append(s101, escaped...)
	s101 = append(s101, crc...)
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
	var crc uint16 = eof16

	reader := bytes.NewReader(data)

	for {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}

		if b == ce {
			next, err := reader.ReadByte()
			if err != nil {
				break
			}

			b = xorce ^ next
		}

		crc = computeCRCByte(crc, b)
	}

	crc = (^crc) & eof16

	return parseCRC([]uint8{uint8(crc & eof), uint8(crc >> checkSumSecondDeviation)})
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
