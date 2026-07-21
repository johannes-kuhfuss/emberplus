/*
** Copyright (C) 2001-2024 Zabbix SIA
** Adaptations (C) 2024 JKU
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

package asn1

import (
	"encoding/asn1"
	"errors"
	"fmt"
	"math"
)

// GetData returns all data contained in the encoder.
func (c *Encoder) GetData() []byte {
	return c.data.Bytes()
}

// WriteRequest writes a request into the encoder buffer, for the provided element type, currently supports parameters,
// qualified parameters, nodes qualified nodes and functions.
func (c *Encoder) WriteRequest(path []int, tag string, cmd int) error {
	c.openSequence(ApplicationByte(RootElementCollectionTag))
	defer c.closeSequence()

	c.openSequence(ApplicationByte(RootElementTag))
	defer c.closeSequence()

	c.openSequence(ContextByte(0))
	defer c.closeSequence()

	switch tag {
	case ParameterType, QualifiedParameterType:
		c.openSequence(ApplicationByte(QualifiedParameterTag))
	case NodeType, QualifiedNodeType:
		c.openSequence(ApplicationByte(QualifiedNodeTag))
	case FunctionType:
		c.openSequence(ApplicationByte(functionTag))
	default:
		return fmt.Errorf("unknown application tag %s", tag)
	}

	defer c.closeSequence()

	c.openSequence(ContextByte(0))
	defer c.closeSequence()

	if err := c.WriteUniversal(path); err != nil {
		return fmt.Errorf("failed to write path: %w", err)
	}

	c.openSequence(ContextByte(2))
	defer c.closeSequence()

	c.openSequence(ApplicationByte(elementCollectionTag))
	defer c.closeSequence()

	err := c.WriteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to writer dir command: %w", err)
	}

	return nil
}

// WriteUniversal writes the provided integer into the buffer as an glow encoded universal value.
func (c *Encoder) WriteUniversal(path []int) error {
	value := make([]byte, 0, len(path))
	for _, component := range path {
		if component < 0 {
			return fmt.Errorf("relative OID component must not be negative: %d", component)
		}

		value = append(value, encodeBase128(uint64(component))...)
	}

	c.data.WriteByte(UniversalObjectTag)
	c.writeLength(len(value))
	c.data.Write(value)

	return nil
}

func encodeBase128(value uint64) []byte {
	if value == 0 {
		return []byte{0}
	}

	var scratch [10]byte
	i := len(scratch)
	for value > 0 {
		i--
		scratch[i] = byte(value & 0x7f)
		if i < len(scratch)-1 {
			scratch[i] |= 0x80
		}
		value >>= 7
	}

	return append([]byte(nil), scratch[i:]...)
}

func (c *Encoder) writeLength(length int) {
	if length < 0x80 {
		c.data.WriteByte(byte(length))
		return
	}

	var scratch [8]byte
	i := len(scratch)
	for value := uint64(length); value > 0; value >>= 8 {
		i--
		scratch[i] = byte(value)
	}

	c.data.WriteByte(0x80 | byte(len(scratch)-i))
	c.data.Write(scratch[i:])
}

// WriteReal writes a BER REAL using the binary, base-2 form required by EmBER.
func (c *Encoder) WriteReal(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return errors.New("EmBER REAL does not support NaN or infinity")
	}
	c.data.WriteByte(0x09)
	if value == 0 {
		c.data.WriteByte(0)
		return nil
	}

	bits := math.Float64bits(value)
	negative := bits>>63 != 0
	exponentBits := int((bits >> 52) & 0x7ff)
	mantissa := bits & ((uint64(1) << 52) - 1)
	var exponent int64
	if exponentBits == 0 {
		exponent = -1074
	} else {
		mantissa |= uint64(1) << 52
		exponent = int64(exponentBits) - 1023 - 52
	}
	for mantissa&1 == 0 {
		mantissa >>= 1
		exponent++
	}

	exponentBytes := encodeSignedInteger(exponent)
	mantissaBytes := encodeUnsignedInteger(mantissa)
	first := byte(0x80)
	if negative {
		first |= 0x40
	}
	switch len(exponentBytes) {
	case 1:
	case 2:
		first |= 0x01
	case 3:
		first |= 0x02
	default:
		first |= 0x03
	}

	contentLength := 1 + len(exponentBytes) + len(mantissaBytes)
	if len(exponentBytes) > 3 {
		contentLength++
	}
	c.writeLength(contentLength)
	c.data.WriteByte(first)
	if len(exponentBytes) > 3 {
		c.data.WriteByte(byte(len(exponentBytes)))
	}
	c.data.Write(exponentBytes)
	c.data.Write(mantissaBytes)
	return nil
}

func encodeSignedInteger(value int64) []byte {
	var scratch [8]byte
	for i := range scratch {
		scratch[len(scratch)-1-i] = byte(value >> (8 * i))
	}
	i := 0
	for i < len(scratch)-1 && ((scratch[i] == 0 && scratch[i+1]&0x80 == 0) || (scratch[i] == 0xff && scratch[i+1]&0x80 != 0)) {
		i++
	}
	return append([]byte(nil), scratch[i:]...)
}

func encodeUnsignedInteger(value uint64) []byte {
	var scratch [8]byte
	i := len(scratch)
	for value > 0 {
		i--
		scratch[i] = byte(value)
		value >>= 8
	}
	if i == len(scratch) {
		return []byte{0}
	}
	return append([]byte(nil), scratch[i:]...)
}

// WriteRootTreeRequest writes a request for root element collection into the buffer.
func (c *Encoder) WriteRootTreeRequest() error {
	c.openSequence(ApplicationByte(RootElementCollectionTag))
	defer c.closeSequence()

	c.openSequence(ApplicationByte(RootElementTag))
	defer c.closeSequence()

	err := c.WriteCommand(EmberGetDirCommand)
	if err != nil {
		return fmt.Errorf("failed to write command request: %w", err)
	}

	return nil
}

// WriteCommand writes a get dir command request into the buffer.
func (c *Encoder) WriteCommand(cmd int) error {
	c.openSequence(ContextByte(0))
	defer c.closeSequence()

	c.openSequence(ApplicationByte(commandApplicationTag))
	defer c.closeSequence()

	err := c.writeInt(cmd, 0)
	if err != nil {
		return fmt.Errorf("failed dir write int: %w", err)
	}

	if cmd == EmberGetDirCommand {
		err = c.writeInt(dirFieldMaskAll, 1)
		if err != nil {
			return fmt.Errorf("failed to write dir field mask int: %w", err)
		}
	}

	return nil
}

// writeInt writes integer to the buffer, wraps native go asn1 marshal, but adds context.
func (c *Encoder) writeInt(i int, cont uint8) error {
	err := c.data.WriteByte(ContextByte(cont))
	if err != nil {
		return fmt.Errorf("failed to write context byte: %w", err)
	}

	b, err := asn1.Marshal(i)
	if err != nil {
		return fmt.Errorf("failed native go int asn1 marshal: %w", err)
	}

	c.writeLength(len(b))
	c.data.Write(b)

	return nil
}

// openSequence writes provided application byte together with a context byte (0x80) into the buffer.
func (c *Encoder) openSequence(appl byte) {
	c.data.WriteByte(appl)
	c.data.WriteByte(contextByte)
}

// closeSequence writes two '0' bytes into the buffer, used to identify end of a sequence.
func (c *Encoder) closeSequence() {
	c.data.Write([]byte{0, 0})
}
