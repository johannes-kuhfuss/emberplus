package asn1

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// TagClass is a BER tag class.
type TagClass uint8

const (
	ClassUniversal TagClass = iota
	ClassApplication
	ClassContext
	ClassPrivate
)

// Tag describes a BER tag independently of its encoded width.
type Tag struct {
	Class       TagClass
	Number      uint64
	Constructed bool
}

// TLV is one decoded BER value. Content excludes the tag and length octets.
type TLV struct {
	Tag        Tag
	Content    []byte
	Children   []TLV
	Indefinite bool
}

// ParseTLV decodes one BER value and returns the unconsumed suffix.
func ParseTLV(data []byte) (TLV, []byte, error) {
	if len(data) == 0 {
		return TLV{}, nil, errors.New("missing BER tag")
	}
	first := data[0]
	tag := Tag{Class: TagClass(first >> 6), Constructed: first&0x20 != 0, Number: uint64(first & 0x1f)}
	index := 1
	if tag.Number == 0x1f {
		tag.Number = 0
		for {
			if index >= len(data) {
				return TLV{}, nil, errors.New("truncated high-tag-number")
			}
			b := data[index]
			index++
			if tag.Number > math.MaxUint64>>7 {
				return TLV{}, nil, errors.New("BER tag number overflows uint64")
			}
			tag.Number = tag.Number<<7 | uint64(b&0x7f)
			if b&0x80 == 0 {
				break
			}
		}
	}
	if index >= len(data) {
		return TLV{}, nil, errors.New("missing BER length")
	}
	lengthFirst := data[index]
	index++
	indefinite := lengthFirst == 0x80
	length := 0
	if !indefinite {
		if lengthFirst&0x80 == 0 {
			length = int(lengthFirst)
		} else {
			lengthBytes := int(lengthFirst & 0x7f)
			if lengthBytes == 0 || lengthBytes > strconv.IntSize/8 || index+lengthBytes > len(data) {
				return TLV{}, nil, errors.New("invalid BER long-form length")
			}
			for _, b := range data[index : index+lengthBytes] {
				if length > (int(^uint(0)>>1) >> 8) {
					return TLV{}, nil, errors.New("BER length overflows int")
				}
				length = length<<8 | int(b)
			}
			index += lengthBytes
		}
		if index+length > len(data) {
			return TLV{}, nil, errors.New("truncated BER content")
		}
	}
	if indefinite && !tag.Constructed {
		return TLV{}, nil, errors.New("primitive BER value uses indefinite length")
	}

	value := TLV{Tag: tag, Indefinite: indefinite}
	if !tag.Constructed {
		value.Content = append([]byte(nil), data[index:index+length]...)
		return value, data[index+length:], nil
	}

	var content []byte
	if indefinite {
		remaining := data[index:]
		for {
			if len(remaining) < 2 {
				return TLV{}, nil, errors.New("unterminated indefinite BER container")
			}
			if remaining[0] == 0 && remaining[1] == 0 {
				value.Content = append([]byte(nil), content...)
				return value, remaining[2:], nil
			}
			child, rest, err := ParseTLV(remaining)
			if err != nil {
				return TLV{}, nil, err
			}
			consumed := len(remaining) - len(rest)
			content = append(content, remaining[:consumed]...)
			value.Children = append(value.Children, child)
			remaining = rest
		}
	}

	content = data[index : index+length]
	remaining := content
	for len(remaining) > 0 {
		child, rest, err := ParseTLV(remaining)
		if err != nil {
			return TLV{}, nil, err
		}
		value.Children = append(value.Children, child)
		remaining = rest
	}
	value.Content = append([]byte(nil), content...)
	return value, data[index+length:], nil
}

// ParseTLVs decodes all BER values in data.
func ParseTLVs(data []byte) ([]TLV, error) {
	var values []TLV
	for len(data) > 0 {
		value, rest, err := ParseTLV(data)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
		data = rest
	}
	return values, nil
}

// Inner returns the value wrapped by an explicitly tagged value.
func (v TLV) Inner() (TLV, error) {
	if len(v.Children) != 1 {
		return TLV{}, fmt.Errorf("explicit BER value has %d children", len(v.Children))
	}
	return v.Children[0], nil
}

// Child returns the first direct child with the requested class and number.
func (v TLV) Child(class TagClass, number uint64) (TLV, bool) {
	for _, child := range v.Children {
		if child.Tag.Class == class && child.Tag.Number == number {
			return child, true
		}
	}
	return TLV{}, false
}

// ExplicitChild returns and unwraps an explicitly tagged direct child.
func (v TLV) ExplicitChild(number uint64) (TLV, bool, error) {
	child, ok := v.Child(ClassContext, number)
	if !ok {
		return TLV{}, false, nil
	}
	inner, err := child.Inner()
	return inner, true, err
}

// Int64 converts a universal BER INTEGER.
func (v TLV) Int64() (int64, error) {
	if v.Tag.Class != ClassUniversal || v.Tag.Number != 2 {
		return 0, errors.New("BER value is not an INTEGER")
	}
	return decodeSignedBytes(v.Content)
}

// Bool converts a universal BER BOOLEAN.
func (v TLV) Bool() (bool, error) {
	if v.Tag.Class != ClassUniversal || v.Tag.Number != 1 || len(v.Content) != 1 {
		return false, errors.New("BER value is not a BOOLEAN")
	}
	return v.Content[0] != 0, nil
}

// String converts a universal BER UTF8String.
func (v TLV) String() (string, error) {
	if v.Tag.Class != ClassUniversal || v.Tag.Number != 12 {
		return "", errors.New("BER value is not a UTF8String")
	}
	return string(v.Content), nil
}

// RelativeOID converts a universal BER RELATIVE-OID.
func (v TLV) RelativeOID() ([]int, error) {
	if v.Tag.Class != ClassUniversal || v.Tag.Number != 13 {
		return nil, errors.New("BER value is not a RELATIVE-OID")
	}
	return NewDecoder(append([]byte{UniversalObjectTag}, appendLengthAndContent(v.Content)...)).DecodeUniversal()
}

// Real converts a universal BER REAL.
func (v TLV) Real() (float64, error) {
	if v.Tag.Class != ClassUniversal || v.Tag.Number != 9 {
		return 0, errors.New("BER value is not a REAL")
	}
	return NewDecoder(append([]byte{0x09}, appendLengthAndContent(v.Content)...)).DecodeReal()
}

func appendLengthAndContent(content []byte) []byte {
	if len(content) < 0x80 {
		return append([]byte{byte(len(content))}, content...)
	}
	var scratch [8]byte
	i := len(scratch)
	for n := len(content); n > 0; n >>= 8 {
		i--
		scratch[i] = byte(n)
	}
	out := []byte{0x80 | byte(len(scratch)-i)}
	out = append(out, scratch[i:]...)
	return append(out, content...)
}

// IsNull reports whether the value is a BER NULL.
func (v TLV) IsNull() bool {
	return v.Tag.Class == ClassUniversal && v.Tag.Number == 5 && len(v.Content) == 0
}
