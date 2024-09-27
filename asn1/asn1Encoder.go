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
	"fmt"
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

	c.WriteUniversal(path)

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
func (c *Encoder) WriteUniversal(path []int) {
	c.data.WriteByte(UniversalObjectTag)
	c.data.WriteByte(uint8(len(path)))

	for _, p := range path {
		c.data.WriteByte(uint8(p))
	}
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

	c.data.WriteByte(uint8(len(b)))
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
