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
	"errors"
	"fmt"
	"io"
)

// Read reads the next glow data block of the appropriate type, it checks the glow tag against the provided compare
// function and if they match it reads the glow data block and returns it as it's own decoded, original decoder might
// have more data left, THIS DOES NOT READ ALL THE DATA.
// If no length byte is found in data returns ALL remaining bytes.
// Returns True if next element length is unknown.
func (c *Decoder) Read(tag uint8, compareByte func(num uint8) uint8) (*Decoder, bool, error) {
	b, err := c.data.ReadByte()
	if err != nil {
		return nil, false, fmt.Errorf("failed to read tag byte: %w", err)
	}

	if b != compareByte(tag) {
		return nil, false, fmt.Errorf("is not correct byte: %x got %x", compareByte(tag), b)
	}

	lenB, _, err := c.readLength()
	if err != nil {
		if !errors.Is(err, ErrNoLenByte) {
			return nil, false, fmt.Errorf("failed to read length byte: %w", err)
		}

		var out []byte

		out, err = c.readWithOutLength()
		if err != nil {
			return nil, false, fmt.Errorf("failed to read with out provided length: %w", err)
		}

		return NewDecoder(out), true, nil
	}

	out, err := c.readWithLength(lenB)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read with provided length: %w", err)
	}

	return NewDecoder(out), false, nil
}

// readLength reads next in line data blocks length and returns it as well as how many bytes the data
// length was written in.
func (c *Decoder) readLength() (int, int, error) {
	lenB, err := c.data.ReadByte()
	if err != nil {
		return 0, 0, fmt.Errorf("incorrect length byte: %w", err)
	}

	if lenB&contextByte != contextByte {
		return int(lenB), 1, nil
	}

	lenB &= lenByte

	if lenB == 0 {
		return 0, 1, ErrNoLenByte
	}

	if lenB > maxLengthBytes {
		return 0, 0, errors.New("length higher than 4")
	}

	var (
		out    int
		offset = 1
	)

	for i := 0; i < int(lenB); i++ {
		val, err := c.data.ReadByte()
		if err != nil {
			return 0, 0, fmt.Errorf("incorrect additional length bytes: %w", err)
		}

		out = out<<8 + int(val)

		offset++
	}

	return out, offset, nil
}

// ReadEnd checks if decoder is currently at the end of element and moves the reader over it, if at end.
func (c *Decoder) ReadEnd() (bool, error) {
	if c.data.Len() == 0 {
		return true, nil
	}

	if c.data.Len() < 2 {
		return false, errors.New("not enough bytes")
	}

	b := c.data.Bytes()
	if b[0] != closingByte || b[1] != closingByte {
		return false, nil
	}

	for i := 0; i < closingOffset; i++ {
		_, err := c.data.ReadByte()
		if err != nil {
			return false, fmt.Errorf("failed to read end bytes: %w", err)
		}
	}

	return true, nil
}

// Peek returns the next byte, but does not remove it from the buffer.
func (c *Decoder) Peek() (byte, error) {
	b, err := c.data.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("failed to read a byte: %w", err)
	}

	err = c.data.UnreadByte()
	if err != nil {
		return 0, fmt.Errorf("failed to unread a byte: %w", err)
	}

	return b, nil
}

// DecodeUniversal decoded the following universal data type of glow, currently only used for universal path decoding,
// witch is an array of integers.
func (c *Decoder) DecodeUniversal() ([]int, error) {
	b, err := c.data.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read tag byte: %w", err)
	}

	if b != UniversalObjectTag {
		return nil, errors.New("incorrect universal byte")
	}

	lenB, _, err := c.readLength()
	if err != nil {
		return nil, fmt.Errorf("failed to read len byte: %w", err)
	}

	var out []int

	for i := 1; i <= lenB; i++ {
		b, err := c.data.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read bytes: %w", err)
		}

		out = append(out, int(b))
	}

	return out, nil
}

// DecodeUTF8 decoded the following utf8 data type of glow.
func (c *Decoder) DecodeUTF8() (string, error) {
	b, err := c.data.ReadByte()
	if err != nil {
		return "", fmt.Errorf("failed to read tag byte: %w", err)
	}

	if b != UTF8StringTag {
		return "", errors.New("incorrect utf8 string byte")
	}

	lenB, _, err := c.readLength()
	if err != nil {
		return "", fmt.Errorf("failed to read len byte: %w", err)
	}

	var out []byte

	for i := 1; i <= lenB; i++ {
		b, err := c.data.ReadByte()
		if err != nil {
			return "", fmt.Errorf("failed to read bytes: %w", err)
		}

		out = append(out, b)
	}

	return string(out), nil
}

// DecodeInteger decodes the following integer.
func (c *Decoder) DecodeInteger() (int, error) {
	t, err := c.data.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("failed to read tag byte: %w", err)
	}

	if t != emberIntTag {
		return 0, errors.New("incorrect integer byte: %w")
	}

	lenB, _, err := c.readLength()
	if err != nil {
		return 0, fmt.Errorf("failed to read len byte: %w", err)
	}

	var out int

	for ; lenB > 0; lenB-- {
		b, err := c.data.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("failed to read extra len bytes: %w", err)
		}

		out = (out << 8) | int(b)
	}

	return out, nil
}

// ReadByte reads one byte from the underlining bytes buffer in decoder.
func (c *Decoder) ReadByte() (byte, error) {
	b, err := c.data.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("failed to read byte: %w", err)
	}

	return b, nil
}

func (c *Decoder) readWithOutLength() ([]byte, error) {
	out, err := io.ReadAll(c.data)
	if err != nil {
		return nil, fmt.Errorf("failed to read all data: %w", err)
	}

	return out, nil
}

func (c *Decoder) readWithLength(length int) ([]byte, error) {
	//nolint:makezero
	out := make([]byte, length)

	n, err := c.data.Read(out)
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes with set length: %w", err)
	}

	if n != length {
		return nil, fmt.Errorf("failed to read bytes with set length, length %d does not match actual read length %d", length, n)
	}

	return out, nil
}
