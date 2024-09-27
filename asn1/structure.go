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
	"bytes"
	"encoding/asn1"
	"errors"
	"fmt"
)

const (
	// Parameter types for Glow parameters.

	// ParameterType glow data field parameter type.
	ParameterType = "parameter"
	// QualifiedParameterType glow data field qualified parameter type.
	QualifiedParameterType = "qualified_parameter"
	// QualifiedNodeType glow data field qualified node type.
	QualifiedNodeType = "qualified_node"
	// NodeType glow data field node type.
	NodeType = "node"
	// FunctionType glow data field function type.
	FunctionType = "function"

	// EmberGetDirCommand integer for request dir command, based on S101 and glow protocol.
	EmberGetDirCommand = 32
	// EmberGetUnsubscribeCommand integer for request Unsubscribe command, based on S101 and glow protocol.
	EmberGetUnsubscribeCommand = 31

	// RootElementCollectionTag tag for defining glow root element collection encoding command.
	RootElementCollectionTag = 0
	// RootElementTag tag for defining glow root element collection.
	RootElementTag = 11
	// ElementCollectionTag tag for element collection.
	ElementCollectionTag = 4
	// ContextZeroTag tag for top level context.
	ContextZeroTag = 0
	// ContextTagOne tag for context
	// Node means data contains content.
	ContextTagOne = 1
	// ContextTagTwo tag for context
	// Node means data contains children.
	ContextTagTwo = 2
	// QualifiedParameterTag for defining glow qualified parameter tag.
	QualifiedParameterTag = 9
	// QualifiedNodeTag for defining glow qualified node tag.
	QualifiedNodeTag = 10

	// SetTag tag for defining Ember set structure.
	SetTag = 49

	// GLOW encoding.

	// UniversalObjectTag context universal object tag.
	UniversalObjectTag = 0x0D
	// UTF8StringTag context utf8String object tag.
	UTF8StringTag = 0x0C
	// byte used for writing context.
	contextByte = 0x80
	// byte used for byte OR check for context tag.
	contextOR = 0xa0
	// byte used for byte OR check for application tag.
	applicationOR = 0x60
	// byte used in glow data len decoding.
	lenByte = 0x7F

	// additional option for dir command, based on S101 and glow protocol.
	dirFieldMaskAll = -1
	// ember encoding int tag.
	emberIntTag = 0x02
	// maximum length of the bytes that describe the data blocks length in glow encoding.
	maxLengthBytes = 4
	// application tag that describes that the glow message is a application command.
	commandApplicationTag = 2
	// tag for defining glow element collection tag.
	elementCollectionTag = 4
	// tag for defining glow function tag.
	functionTag = 20

	// tag for defining glow offset when reading all values.
	closingOffset = 2
	// closingByte defines the closing byte of any glow element.
	closingByte = 0x00
)

// ErrNoLenByte  error when length of bytes can not be determined.
var ErrNoLenByte = errors.New("can not determine length")

// Decoder decoder for ASN1 glow data.
type Decoder struct {
	data *bytes.Buffer
}

// Encoder encoder ASN1 glow data.
type Encoder struct {
	data *bytes.Buffer
}

// NewDecoder creates a new ASN1 Decoder.
func NewDecoder(b []byte) *Decoder {
	return &Decoder{bytes.NewBuffer(b)}
}

// Bytes wrapper to containing decoders data bytes.
func (c *Decoder) Bytes() []byte {
	if c == nil || c.data == nil {
		return nil
	}

	return c.data.Bytes()
}

// Len Bytes wrapper to containing decoders data bytes.
func (c *Decoder) Len() int {
	if c == nil || c.data == nil {
		return 0
	}

	return c.data.Len()
}

// NewEncoder creates a new encoder with an initialized data buffer, but no actual data.
func NewEncoder() *Encoder {
	return &Encoder{bytes.NewBuffer(nil)}
}

// DecodeAny decodes native asn1 value.
func DecodeAny(in []byte, val any) (int, error) {
	slen := len(in)

	r, err := asn1.Unmarshal(in, val)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal go native asn1 value: %w", err)
	}

	return slen - len(r), nil
}

// tag types used in ember+ glow protocol.

// ApplicationByte have the same meaning wherever they are seen and used.
func ApplicationByte(num uint8) uint8 {
	return applicationOR | num
}

// ContextByte context-specific tags depends on the location where they are seen.
func ContextByte(num uint8) uint8 {
	return contextOR | num
}

// UniversalByte predefined types, the value is returned unchanged.
func UniversalByte(num uint8) uint8 {
	return num
}
