package asn1

import (
	"errors"
	"fmt"
)

// MarshalTLV encodes a definite-length BER value.
func MarshalTLV(tag Tag, content []byte) ([]byte, error) {
	tagBytes, err := marshalTag(tag)
	if err != nil {
		return nil, err
	}
	out := append([]byte(nil), tagBytes...)
	out = append(out, marshalLength(len(content))...)
	return append(out, content...), nil
}

// MarshalContainer encodes a constructed BER value containing children.
func MarshalContainer(class TagClass, number uint64, children ...[]byte) ([]byte, error) {
	var content []byte
	for _, child := range children {
		content = append(content, child...)
	}
	return MarshalTLV(Tag{Class: class, Number: number, Constructed: true}, content)
}

// MarshalExplicit applies a constructed context-specific explicit tag.
func MarshalExplicit(number uint64, inner []byte) ([]byte, error) {
	return MarshalContainer(ClassContext, number, inner)
}

// MarshalInteger encodes a universal signed BER INTEGER.
func MarshalInteger(value int64) []byte {
	out, _ := MarshalTLV(Tag{Class: ClassUniversal, Number: 2}, encodeSignedInteger(value))
	return out
}

// MarshalBoolean encodes a universal BER BOOLEAN.
func MarshalBoolean(value bool) []byte {
	b := byte(0)
	if value {
		b = 0xff
	}
	out, _ := MarshalTLV(Tag{Class: ClassUniversal, Number: 1}, []byte{b})
	return out
}

// MarshalString encodes a universal BER UTF8String.
func MarshalString(value string) []byte {
	out, _ := MarshalTLV(Tag{Class: ClassUniversal, Number: 12}, []byte(value))
	return out
}

// MarshalOctets encodes a universal BER OCTET STRING.
func MarshalOctets(value []byte) []byte {
	out, _ := MarshalTLV(Tag{Class: ClassUniversal, Number: 4}, value)
	return out
}

// MarshalNull encodes a universal BER NULL.
func MarshalNull() []byte { return []byte{0x05, 0x00} }

// MarshalRelativeOID encodes a universal BER RELATIVE-OID.
func MarshalRelativeOID(path []int) ([]byte, error) {
	var content []byte
	for _, component := range path {
		if component < 0 {
			return nil, fmt.Errorf("relative OID component must not be negative: %d", component)
		}
		content = append(content, encodeBase128(uint64(component))...)
	}
	return MarshalTLV(Tag{Class: ClassUniversal, Number: 13}, content)
}

// MarshalReal encodes the EmBER binary REAL subset.
func MarshalReal(value float64) ([]byte, error) {
	encoder := NewEncoder()
	if err := encoder.WriteReal(value); err != nil {
		return nil, err
	}
	return append([]byte(nil), encoder.GetData()...), nil
}

func marshalTag(tag Tag) ([]byte, error) {
	if tag.Class > ClassPrivate {
		return nil, errors.New("invalid BER tag class")
	}
	first := byte(tag.Class << 6)
	if tag.Constructed {
		first |= 0x20
	}
	if tag.Number < 31 {
		return []byte{first | byte(tag.Number)}, nil
	}
	return append([]byte{first | 0x1f}, encodeBase128(tag.Number)...), nil
}

func marshalLength(length int) []byte {
	if length < 0x80 {
		return []byte{byte(length)}
	}
	var scratch [8]byte
	i := len(scratch)
	for length > 0 {
		i--
		scratch[i] = byte(length)
		length >>= 8
	}
	return append([]byte{0x80 | byte(len(scratch)-i)}, scratch[i:]...)
}
