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

package ember

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/johannes-kuhfuss/emberplus/asn1"
)

const (
	// tag for defining glow node tag.
	nodeTag = 3
	// tag for defining glow function tag.
	functionTag = 20
	// parameterTag glow  parameter tag.
	parameterTag = 1

	// node values types held in context(13), define what is the type of value in context(2).
	valueTypeInt    = 1
	valueTypeReal   = 2
	valueTypeString = 3
	valueTypeBool   = 4
	valueTypeEnum   = 6
)

// ErrElementNotFound error when element is not found.
var ErrElementNotFound = errors.New("element not found")

// node hold information about node and qualified node parameter fields.
type node struct {
	Path        string      `json:"path"`
	ElementType ElementType `json:"element_type"`
	Children    []*Element  `json:"children"`
	Identifier  string      `json:"identifier"`
	Description string      `json:"description"`
	IsOnline    bool        `json:"is_online"`
	IsRoot      bool        `json:"is_root"`
}

// function hold information about function parameter fields.
type function struct {
	Path        string      `json:"path"`
	ElementType ElementType `json:"element_type"`
	Children    []*Element  `json:"children"`
	Identifier  string      `json:"identifier"`
	Description string      `json:"description"`
}

// parameter hold information about parameter and qualified parameter fields.
type parameter struct {
	Path        string      `json:"path"`
	ElementType ElementType `json:"element_type"`
	Children    []*Element  `json:"children,omitempty"`
	Identifier  string      `json:"identifier,omitempty"`
	Description string      `json:"description,omitempty"`
	Value       any         `json:"value,omitempty"`
	Minimum     any         `json:"minimum,omitempty"`
	Maximum     any         `json:"maximum,omitempty"`
	Access      int         `json:"access,omitempty"`
	Format      string      `json:"format,omitempty"`
	Enumeration string      `json:"enumeration,omitempty"`
	Factor      int         `json:"factor,omitempty"`
	IsOnline    bool        `json:"is_online,omitempty"`
	Default     any         `json:"default,omitempty"`
	ValueType   int         `json:"type,omitempty"`
}

// ElementType wrapper for string to define available element types.
type ElementType string

// Element contains all the values a glow element might contain.
type Element struct {
	Path        string
	ElementType ElementType
	Identifier  string
	Description string
	Children    []*Element
	IsOnline    bool
	IsRoot      bool
	Maximum     any
	Minimum     any
	Value       any
	Access      int
	Format      string
	Enumeration string
	Factor      int
	Default     any
	ValueType   int
}

//nolint:gocyclo,cyclop
func (el *Element) handleApplication(decoder *asn1.Decoder) (*asn1.Decoder, error) {
	for {
		t, err := decoder.Peek()
		if err != nil {
			return nil, fmt.Errorf("failed to peek context: %w", err)
		}

		var decoders []*asn1.Decoder

		switch asn1.ContextByte(t) {
		case asn1.ContextByte(asn1.ContextZeroTag):
			decoders, err = el.handlePath(decoder)
			if err != nil {
				return nil, fmt.Errorf("failed to read path: %w", err)
			}
		case asn1.ContextByte(asn1.ContextTagOne):
			decoders, err = el.handleContent(decoder)
			if err != nil {
				return nil, fmt.Errorf("failed to read content: %w", err)
			}
		case asn1.ContextByte(asn1.ContextTagTwo):
			decoders, err = el.handleChildren(decoder)
			if err != nil {
				return nil, fmt.Errorf("failed to read children: %w", err)
			}
		}

		decoder, err = findWithData(decoders)
		if err != nil {
			return nil, fmt.Errorf("failed to find the decoder to continue reading: %w", err)
		}

		atEnd, err := decoder.ReadEnd()
		if err != nil {
			return nil, fmt.Errorf("failed to read end bytes: %w", err)
		}

		if atEnd {
			return decoder, nil
		}
	}
}

func (el *Element) handleChildren(decoder *asn1.Decoder) ([]*asn1.Decoder, error) {
	anyDec, _, err := decoder.Read(asn1.ContextByte(asn1.ContextTagTwo), asn1.ContextByte)
	if err != nil {
		return nil, fmt.Errorf("failed to read child context: %w", err)
	}

	childDec, _, err := anyDec.Read(asn1.ApplicationByte(asn1.ElementCollectionTag), asn1.ApplicationByte)
	if err != nil {
		return nil, fmt.Errorf("failed to get children elements: %w", err)
	}

	for {
		var decoders []*asn1.Decoder

		decoders, err = el.setChild(childDec)
		if err != nil {
			return nil, fmt.Errorf("failed to set child element: %w", err)
		}

		childDec, err = findWithData(decoders)
		if err != nil {
			return nil, fmt.Errorf("failed to find the decoder to continue reading: %w", err)
		}

		_, err := childDec.ReadEnd() // current child context end
		if err != nil {
			return nil, fmt.Errorf("failed to decode child context end: %w", err)
		}

		_, err = childDec.ReadEnd() // current child elements end
		if err != nil {
			return nil, fmt.Errorf("failed to decode current child sequence end: %w", err)
		}

		end, err := childDec.ReadEnd() // all child element end
		if err != nil {
			return nil, fmt.Errorf("failed to decode child element sequence end: %w", err)
		}

		if end {
			break
		}
	}

	return []*asn1.Decoder{decoder, anyDec, childDec}, nil
}

func (el *Element) setChild(childrenDecoder *asn1.Decoder) ([]*asn1.Decoder, error) {
	allChild, _, err := childrenDecoder.Read(asn1.ContextByte(asn1.ContextZeroTag), asn1.ContextByte)
	if err != nil {
		return nil, fmt.Errorf("failed to read next child element: %w", err)
	}

	child, tmp, err := getElement(allChild)
	if err != nil {
		return nil, fmt.Errorf("failed to decode next child element: %w", err)
	}

	el.Children = append(el.Children, child)

	return []*asn1.Decoder{childrenDecoder, allChild, tmp}, nil
}

func (el *Element) handleContent(decoder *asn1.Decoder) ([]*asn1.Decoder, error) {
	content, _, err := decoder.Read(asn1.ContextByte(asn1.ContextTagOne), asn1.ContextByte)
	if err != nil {
		return nil, fmt.Errorf("failed to read context: %w", err)
	}

	set, _, err := content.Read(asn1.SetTag, asn1.UniversalByte)
	if err != nil {
		return nil, fmt.Errorf("failed to read set: %w", err)
	}

	for {
		var decoders []*asn1.Decoder

		decoders, err = el.handleContentContext(set)
		if err != nil {
			return nil, fmt.Errorf("failed to handle content: %w", err)
		}

		set, err = findWithData(decoders)
		if err != nil {
			return nil, fmt.Errorf("failed to find the decoder to continue reading: %w", err)
		}

		end, err := set.ReadEnd()
		if err != nil {
			return nil, fmt.Errorf("failed to read sequence end: %w", err)
		}

		if end {
			break
		}
	}

	return []*asn1.Decoder{decoder, content, set}, nil
}

func (el *Element) handleContentContext(decoder *asn1.Decoder) ([]*asn1.Decoder, error) {
	t, err := decoder.Peek()
	if err != nil {
		return nil, fmt.Errorf("failed to peek context byte: %w", err)
	}

	context, _, err := decoder.Read(t, asn1.ContextByte)
	if err != nil {
		return nil, fmt.Errorf("failed to read context: %w", err)
	}

	switch el.ElementType {
	case asn1.QualifiedParameterType, asn1.ParameterType:
		context, err = el.handleParameterContext(context, t)
		if err != nil {
			return nil, fmt.Errorf("failed to decode parameter context: %w", err)
		}
	case asn1.NodeType, asn1.QualifiedNodeType:
		context, err = el.handleNodeContext(context, t)
		if err != nil {
			return nil, fmt.Errorf("failed to decode node context: %w", err)
		}
	case asn1.FunctionType:
		context, err = el.handleFunctionContext(context, t)
		if err != nil {
			return nil, fmt.Errorf("failed to decode function context: %w", err)
		}
	}

	return []*asn1.Decoder{decoder, context}, nil
}

func (el *Element) handlePath(decoder *asn1.Decoder) ([]*asn1.Decoder, error) {
	contextDec, _, err := decoder.Read(asn1.ContextZeroTag, asn1.ContextByte)
	if err != nil {
		return nil, fmt.Errorf("failed to context: %w", err)
	}

	path, err := getPath(contextDec)
	if err != nil {
		return nil, fmt.Errorf("failed to get path: %w", err)
	}

	el.Path = path

	atEnd, err := contextDec.ReadEnd()
	if err != nil {
		return nil, fmt.Errorf("failed to read end bytes: %w", err)
	}

	if !atEnd {
		return nil, errors.New("not at sequence")
	}

	return []*asn1.Decoder{decoder, contextDec}, nil
}

func getPath(decoder *asn1.Decoder) (string, error) {
	tag, err := decoder.Peek()
	if err != nil {
		return "", fmt.Errorf("failed to peek path byte: %w", err)
	}

	if tag == asn1.UniversalObjectTag {
		var path string

		path, err = handlePathFromUniversal(decoder)
		if err != nil {
			return "", fmt.Errorf("failed to read path from universal: %w", err)
		}

		return path, nil
	}

	p, err := decoder.DecodeInteger()
	if err != nil {
		return "", fmt.Errorf("failed to handle path context: %w", err)
	}

	return strconv.Itoa(p), nil
}

// getElement reads next full element from the decoder and returns leftover decoder.
func getElement(d *asn1.Decoder) (*Element, *asn1.Decoder, error) {
	t, err := d.Peek()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read context: %w", err)
	}

	el := &Element{}

	decoder, _, err := d.Read(t, asn1.ApplicationByte)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read element application: %w", err)
	}

	switch asn1.ApplicationByte(t) {
	case asn1.ApplicationByte(asn1.QualifiedNodeTag):
		el.ElementType = asn1.QualifiedNodeType
	case asn1.ApplicationByte(asn1.QualifiedParameterTag):
		el.ElementType = asn1.QualifiedParameterType
	case asn1.ApplicationByte(nodeTag):
		el.ElementType = asn1.NodeType
	case asn1.ApplicationByte(parameterTag):
		el.ElementType = asn1.ParameterType
	case asn1.ApplicationByte(functionTag):
		el.ElementType = asn1.FunctionType
	default:
		return nil, nil, fmt.Errorf("unknown type: %x", t)
	}

	decoder, err = el.handleApplication(decoder)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to handle application with type %x: %w", asn1.ApplicationByte(t), err)
	}

	return el, decoder, nil
}

//nolint:gocyclo,cyclop
func (el *Element) handleFunctionContext(context *asn1.Decoder, tag byte) (*asn1.Decoder, error) {
	var (
		n   int
		err error
	)

	switch asn1.ContextByte(tag) {
	case asn1.ContextByte(0):
		var id string

		id, err = context.DecodeUTF8()
		if err != nil {
			return nil, fmt.Errorf("failed to decode identifier: %w", err)
		}

		el.Identifier = id
	case asn1.ContextByte(1):
		var desc string

		desc, err = context.DecodeUTF8()
		if err != nil {
			return nil, fmt.Errorf("failed to decode description: %w", err)
		}

		el.Description = desc
	case asn1.ContextByte(2):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(2), err)
		}
	case asn1.ContextByte(3):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(3), err)
		}
	}

	for i := 0; i < n; i++ {
		_, err := context.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read over used bytes: %w", err)
		}
	}

	return context, nil
}

//nolint:gocyclo,cyclop
func (el *Element) handleNodeContext(context *asn1.Decoder, tag byte) (*asn1.Decoder, error) {
	var (
		n   int
		err error
	)

	switch asn1.ContextByte(tag) {
	case asn1.ContextByte(0):
		var id string

		id, err = context.DecodeUTF8()
		if err != nil {
			return nil, fmt.Errorf("failed to decode identifier: %w", err)
		}

		el.Identifier = id
	case asn1.ContextByte(1):
		var desc string

		desc, err = context.DecodeUTF8()
		if err != nil {
			return nil, fmt.Errorf("failed to decode description: %w", err)
		}

		el.Description = desc
	case asn1.ContextByte(2):
		var root bool

		n, err = asn1.DecodeAny(context.Bytes(), &root)
		if err != nil {
			return nil, fmt.Errorf("failed to decode is root: %w", err)
		}

		el.IsRoot = root
	case asn1.ContextByte(3):
		var online bool

		n, err = asn1.DecodeAny(context.Bytes(), &online)
		if err != nil {
			return nil, fmt.Errorf("failed to decode is online: %w", err)
		}

		el.IsOnline = online
	case asn1.ContextByte(4):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(4), err)
		}
	case asn1.ContextByte(5):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(5), err)
		}
	}

	for i := 0; i < n; i++ {
		_, err := context.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read over used bytes: %w", err)
		}
	}

	return context, nil
}

// handlePropertyContext decodes context property tag.
//
//nolint:gocognit,gocyclo,cyclop
func (el *Element) handleParameterContext(context *asn1.Decoder, tag byte) (*asn1.Decoder, error) {
	var (
		n   int
		err error
	)

	switch asn1.ContextByte(tag) {
	case asn1.ContextByte(0):
		var id string

		id, err = context.DecodeUTF8()
		if err != nil {
			return nil, fmt.Errorf("failed to decode identifier: %w", err)
		}

		el.Identifier = id
	case asn1.ContextByte(1):
		var desc string

		desc, err = context.DecodeUTF8()
		if err != nil {
			return nil, fmt.Errorf("failed to decode description: %w", err)
		}

		el.Description = desc
	case asn1.ContextByte(2):
		var value any

		value, n, err = el.setValue(context)
		if err != nil {
			return nil, fmt.Errorf("failed to decode parameter value: %w", err)
		}

		el.Value = value
	case asn1.ContextByte(3):
		var min any

		n, err = asn1.DecodeAny(context.Bytes(), &min)
		if err != nil {
			return nil, fmt.Errorf("failed to decode is min: %w", err)
		}

		el.Minimum = min
	case asn1.ContextByte(4):
		var max any

		n, err = asn1.DecodeAny(context.Bytes(), &max)
		if err != nil {
			return nil, fmt.Errorf("failed to decode is max: %w", err)
		}

		el.Maximum = max
	case asn1.ContextByte(5):
		var access int

		access, err = context.DecodeInteger()
		if err != nil {
			return nil, fmt.Errorf("failed to decode is access: %w", err)
		}

		el.Access = access
	case asn1.ContextByte(6):
		var format string

		format, err = context.DecodeUTF8()
		if err != nil {
			return nil, fmt.Errorf("failed to decode format: %w", err)
		}

		el.Format = format
	case asn1.ContextByte(7):
		var enum string

		enum, err = context.DecodeUTF8()
		if err != nil {
			return nil, fmt.Errorf("failed to decode enumeration: %w", err)
		}

		el.Enumeration = enum
	case asn1.ContextByte(8):
		var factor int

		factor, err = context.DecodeInteger()
		if err != nil {
			return nil, fmt.Errorf("failed to decode is factor: %w", err)
		}

		el.Factor = factor
	case asn1.ContextByte(9):
		var online bool

		n, err = asn1.DecodeAny(context.Bytes(), &online)
		if err != nil {
			return nil, fmt.Errorf("failed to decode is online: %w", err)
		}

		el.IsOnline = online
	case asn1.ContextByte(10):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(10), err)
		}
	case asn1.ContextByte(11):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(11), err)
		}
	case asn1.ContextByte(12):
		var def any

		n, err = asn1.DecodeAny(context.Bytes(), &def)
		if err != nil {
			return nil, fmt.Errorf("failed to decode default value: %w", err)
		}

		el.Default = def
	case asn1.ContextByte(13):
		var valType int

		valType, err = context.DecodeInteger()
		if err != nil {
			return nil, fmt.Errorf("failed to decode default value: %w", err)
		}

		el.ValueType = valType

	case asn1.ContextByte(14):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(14), err)
		}
	case asn1.ContextByte(15):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(15), err)
		}
	case asn1.ContextByte(16):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(16), err)
		}
	case asn1.ContextByte(17):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(17), err)
		}
	case asn1.ContextByte(18):
		context, err = readOverElement(context)
		if err != nil {
			return nil, fmt.Errorf("failed to skip element at %x: %w", asn1.ContextByte(18), err)
		}
	}

	el.setDefaultElementValue()

	for i := 0; i < n; i++ {
		_, err := context.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read over used bytes: %w", err)
		}
	}

	return context, nil
}

// setValue set's element value based on value type, this function assumes that value type is already set for element
// as it comes before value in data stream, but it will try to decode a default value type is none is set.
func (el *Element) setValue(context *asn1.Decoder) (any, int, error) {
	var (
		out any
		n   int
		err error
	)

	switch el.ValueType {
	case valueTypeInt, valueTypeEnum:
		out, err = context.DecodeInteger()
		if err != nil {
			return nil, 0, errors.New("failed to decode integer value")
		}
	case valueTypeString:
		out, err = context.DecodeUTF8()
		if err != nil {
			return nil, 0, errors.New("failed to decode string value")
		}
	case valueTypeBool:
		var b bool

		n, err = asn1.DecodeAny(context.Bytes(), &b)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to decode value of type %d: %w", el.ValueType, err)
		}

		out = b
	default:
		n, err = asn1.DecodeAny(context.Bytes(), &out)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to decode value of type %d: %w", el.ValueType, err)
		}
	}

	return out, n, nil
}

func (el *Element) setDefaultElementValue() {
	// no default for enum data type as value for enum data type defines witch of string lines in enum field to use.
	// and none should be used if there no value
	if el.Value == nil {
		switch el.ValueType {
		case valueTypeInt, valueTypeReal:
			el.Value = 0
		case valueTypeString:
			el.Value = ""
		case valueTypeBool:
			el.Value = false
		}
	}
}

func findWithData(decoders []*asn1.Decoder) (*asn1.Decoder, error) {
	var out *asn1.Decoder
	for _, d := range decoders {
		if out != nil && d.Len() > 0 {
			return nil, errors.New("after value handling both new and original decoders have data left")
		}

		if d.Len() > 0 {
			out = d

			continue
		}
	}

	if out != nil {
		return out, nil
	}

	return asn1.NewDecoder([]byte{}), nil
}

func handlePathFromUniversal(dec *asn1.Decoder) (string, error) {
	path, err := dec.DecodeUniversal()
	if err != nil {
		return "", fmt.Errorf("failed to decode integer: %w", err)
	}

	strPath := make([]string, 0, len(path))

	for _, p := range path {
		strPath = append(strPath, strconv.Itoa(p))
	}

	return strings.Join(strPath, "."), nil
}

// readOverElement skips next element in decoder.
func readOverElement(decoder *asn1.Decoder) (*asn1.Decoder, error) {
	tag, err := decoder.Peek()
	if err != nil {
		return nil, fmt.Errorf("failed to peek next element tag")
	}

	newDec, allDataInNew, err := decoder.Read(tag, asn1.UniversalByte)
	if err != nil {
		return nil, fmt.Errorf("failed to read next element")
	}

	if !allDataInNew {
		return decoder, nil
	}

	for {
		_, err := newDec.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("failed to read next element bytes")
		}

		end, err := newDec.ReadEnd()
		if err != nil {
			return nil, fmt.Errorf("failed to read next element end")
		}

		if end {
			return newDec, nil
		}
	}
}

// parsePath returns string oid path as integer array.
func parsePath(path string) ([]int, error) {
	if path == "" {
		return nil, nil
	}

	paths := strings.Split(path, ".")
	out := make([]int, 0, len(paths))

	for _, p := range paths {
		i, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("failed to parse path component")
		}

		out = append(out, i)
	}

	return out, nil
}
