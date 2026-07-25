package ember

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	ber "github.com/johannes-kuhfuss/emberplus/asn1"
)

const (
	ParameterElement          ElementType = "parameter"
	QualifiedParameterElement ElementType = "qualified_parameter"
	NodeElement               ElementType = "node"
	QualifiedNodeElement      ElementType = "qualified_node"
	MatrixElement             ElementType = "matrix"
	QualifiedMatrixElement    ElementType = "qualified_matrix"
	FunctionElement           ElementType = "function"
	QualifiedFunctionElement  ElementType = "qualified_function"
	CommandElement            ElementType = "command"
	TemplateElement           ElementType = "template"
	QualifiedTemplateElement  ElementType = "qualified_template"
)

// EnumEntry describes one Glow enumeration mapping.
type EnumEntry struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// StreamDescriptor describes the binary representation of a streamed value.
type StreamDescriptor struct {
	Format int64 `json:"format"`
	Offset int64 `json:"offset"`
}

// Matrix contains the matrix-specific parts of a Glow element.
type Matrix struct {
	Type                     int64              `json:"type"`
	AddressingMode           int64              `json:"addressing_mode"`
	TargetCount              int64              `json:"target_count"`
	SourceCount              int64              `json:"source_count"`
	MaximumTotalConnects     int64              `json:"maximum_total_connects,omitempty"`
	MaximumConnectsPerTarget int64              `json:"maximum_connects_per_target,omitempty"`
	ParametersLocation       any                `json:"parameters_location,omitempty"`
	GainParameterNumber      int64              `json:"gain_parameter_number,omitempty"`
	Labels                   []Label            `json:"labels,omitempty"`
	Targets                  []int64            `json:"targets,omitempty"`
	Sources                  []int64            `json:"sources,omitempty"`
	Connections              []MatrixConnection `json:"connections,omitempty"`
}

// Label gives a matrix label path a human-readable description.
type Label struct {
	BasePath    string `json:"base_path"`
	Description string `json:"description"`
}

// MatrixConnection represents a target and its connected sources.
type MatrixConnection struct {
	Target      int64 `json:"target"`
	Sources     []int `json:"sources,omitempty"`
	Operation   int64 `json:"operation,omitempty"`
	Disposition int64 `json:"disposition,omitempty"`
}

// TupleItem describes one function argument or result item.
type TupleItem struct {
	Type int64  `json:"type"`
	Name string `json:"name,omitempty"`
}

// Function describes the signature of a Glow function.
type Function struct {
	Arguments []TupleItem `json:"arguments,omitempty"`
	Result    []TupleItem `json:"result,omitempty"`
}

// Template describes a reusable Glow element template.
type Template struct {
	Description string   `json:"description,omitempty"`
	Element     *Element `json:"element,omitempty"`
}

// Command is a Glow command and its optional arguments.
type Command struct {
	Number       int64       `json:"number"`
	DirFieldMask int64       `json:"dir_field_mask,omitempty"`
	Invocation   *Invocation `json:"invocation,omitempty"`
}

// Invocation requests that a Glow function be called.
type Invocation struct {
	ID        int64 `json:"id,omitempty"`
	Arguments []any `json:"arguments,omitempty"`
}

// InvocationResult reports the result of a Glow function invocation.
type InvocationResult struct {
	ID      int64 `json:"id"`
	Success bool  `json:"success"`
	Result  []any `json:"result,omitempty"`
}

// StreamEntry is one value in a Glow stream collection.
type StreamEntry struct {
	Identifier int64 `json:"identifier"`
	Value      any   `json:"value"`
}

// RootMessage is any legal Glow root payload.
type RootMessage struct {
	Elements         ElementCollection `json:"elements,omitempty"`
	Streams          []StreamEntry     `json:"streams,omitempty"`
	InvocationResult *InvocationResult `json:"invocation_result,omitempty"`
}

// DecodeRoot decodes a Glow 2.50 root message.
func DecodeRoot(data []byte) (RootMessage, error) {
	root, rest, err := ber.ParseTLV(data)
	if err != nil {
		return RootMessage{}, err
	}
	if len(rest) != 0 {
		return RootMessage{}, errors.New("Glow root has trailing data")
	}
	if root.Tag.Class != ber.ClassApplication || root.Tag.Number != 0 {
		return RootMessage{}, errors.New("Glow message does not start with application tag 0")
	}
	if len(root.Children) != 1 {
		return RootMessage{}, errors.New("Glow root must contain exactly one value")
	}

	value := root.Children[0]
	if value.Tag.Class != ber.ClassApplication {
		return RootMessage{}, errors.New("Glow root value is not application tagged")
	}
	switch value.Tag.Number {
	case 11:
		collection, err := decodeElementCollection(value, true)
		return RootMessage{Elements: collection}, err
	case 6:
		streams, err := decodeStreams(value)
		return RootMessage{Streams: streams}, err
	case 23:
		result, err := decodeInvocationResult(value)
		return RootMessage{InvocationResult: result}, err
	default:
		return RootMessage{}, fmt.Errorf("unsupported Glow root application tag %d", value.Tag.Number)
	}
}

func decodeElementCollection(value ber.TLV, root bool) (ElementCollection, error) {
	expected := uint64(4)
	if root {
		expected = 11
	}
	if value.Tag.Class != ber.ClassApplication || value.Tag.Number != expected {
		return nil, fmt.Errorf("expected Glow collection application tag %d", expected)
	}
	out := NewElementCollection()
	for _, entry := range value.Children {
		if entry.Tag.Class != ber.ClassContext || entry.Tag.Number != 0 {
			return nil, errors.New("Glow collection entry is not context tag 0")
		}
		inner, err := entry.Inner()
		if err != nil {
			return nil, err
		}
		element, err := decodeElement(inner)
		if err != nil {
			return nil, err
		}
		out[ElementKey{ID: element.Identifier, Path: element.Path}] = element
	}
	return out, nil
}

func decodeElement(value ber.TLV) (*Element, error) {
	if value.Tag.Class != ber.ClassApplication {
		return nil, errors.New("Glow element is not application tagged")
	}
	el := &Element{}
	qualified := false
	switch value.Tag.Number {
	case 1:
		el.ElementType = ParameterElement
	case 9:
		el.ElementType, qualified = QualifiedParameterElement, true
	case 3:
		el.ElementType = NodeElement
	case 10:
		el.ElementType, qualified = QualifiedNodeElement, true
	case 13:
		el.ElementType, el.Matrix = MatrixElement, &Matrix{}
	case 17:
		el.ElementType, el.Matrix, qualified = QualifiedMatrixElement, &Matrix{}, true
	case 19:
		el.ElementType, el.Function = FunctionElement, &Function{}
	case 20:
		el.ElementType, el.Function, qualified = QualifiedFunctionElement, &Function{}, true
	case 2:
		el.ElementType, el.Command = CommandElement, &Command{}
	case 24:
		el.ElementType, el.Template = TemplateElement, &Template{}
	case 25:
		el.ElementType, el.Template, qualified = QualifiedTemplateElement, &Template{}, true
	default:
		return nil, fmt.Errorf("unsupported Glow element application tag %d", value.Tag.Number)
	}

	if el.Command != nil {
		return decodeCommand(value, el)
	}
	if pathValue, ok, err := value.ExplicitChild(0); err != nil {
		return nil, err
	} else if ok {
		if qualified {
			path, err := pathValue.RelativeOID()
			if err != nil {
				return nil, err
			}
			el.Path = formatPath(path)
		} else {
			number, err := pathValue.Int64()
			if err != nil {
				return nil, err
			}
			el.Path = strconv.FormatInt(number, 10)
		}
	}

	if el.Template != nil {
		return decodeTemplate(value, el)
	}
	if contents, ok, err := value.ExplicitChild(1); err != nil {
		return nil, err
	} else if ok {
		if err := decodeContents(el, contents); err != nil {
			return nil, err
		}
		finalizeParameterType(el)
	}
	if children, ok, err := value.ExplicitChild(2); err != nil {
		return nil, err
	} else if ok {
		for _, entry := range children.Children {
			inner, err := entry.Inner()
			if err != nil {
				return nil, err
			}
			child, err := decodeElement(inner)
			if err != nil {
				return nil, err
			}
			el.Children = append(el.Children, child)
		}
	}
	if el.Matrix != nil {
		if err := decodeMatrixCollections(value, el.Matrix); err != nil {
			return nil, err
		}
	}
	return el, nil
}

func decodeContents(el *Element, contents ber.TLV) error {
	for _, field := range contents.Children {
		if field.Tag.Class != ber.ClassContext {
			continue
		}
		inner, err := field.Inner()
		if err != nil {
			return err
		}
		switch el.ElementType {
		case ParameterElement, QualifiedParameterElement:
			err = decodeParameterField(el, field.Tag.Number, inner)
		case NodeElement, QualifiedNodeElement:
			err = decodeNodeField(el, field.Tag.Number, inner)
		case MatrixElement, QualifiedMatrixElement:
			err = decodeMatrixField(el, field.Tag.Number, inner)
		case FunctionElement, QualifiedFunctionElement:
			err = decodeFunctionField(el, field.Tag.Number, inner)
		}
		if err != nil {
			return fmt.Errorf("failed to decode contents field %d: %w", field.Tag.Number, err)
		}
	}
	return nil
}

func decodeParameterField(el *Element, tag uint64, value ber.TLV) error {
	var err error
	switch tag {
	case 0:
		el.Identifier, err = value.String()
	case 1:
		el.Description, err = value.String()
	case 2:
		el.HasValue = true
		el.Value, err = decodeValue(value)
	case 3:
		el.Minimum, err = decodeValue(value)
	case 4:
		el.Maximum, err = decodeValue(value)
	case 5:
		el.Access, err = intValue(value)
	case 6:
		el.Format, err = value.String()
	case 7:
		el.Enumeration, err = value.String()
	case 8:
		el.Factor, err = intValue(value)
	case 9:
		el.IsOnline, err = value.Bool()
	case 10:
		el.Formula, err = value.String()
	case 11:
		el.Step, err = value.Int64()
	case 12:
		el.HasDefault = true
		el.Default, err = decodeValue(value)
	case 13:
		el.HasValueType = true
		el.ValueType, err = intValue(value)
	case 14:
		el.StreamIdentifier, err = value.Int64()
	case 15:
		el.EnumMap, err = decodeEnumMap(value)
	case 16:
		el.StreamDescriptor, err = decodeStreamDescriptor(value)
	case 17:
		el.SchemaIdentifiers, err = value.String()
	case 18:
		var path []int
		path, err = value.RelativeOID()
		el.TemplateReference = formatPath(path)
	default:
		storeUnknown(el, tag, value)
	}
	return err
}

func decodeNodeField(el *Element, tag uint64, value ber.TLV) error {
	var err error
	switch tag {
	case 0:
		el.Identifier, err = value.String()
	case 1:
		el.Description, err = value.String()
	case 2:
		el.IsRoot, err = value.Bool()
	case 3:
		el.IsOnline, err = value.Bool()
	case 4:
		el.SchemaIdentifiers, err = value.String()
	case 5:
		var path []int
		path, err = value.RelativeOID()
		el.TemplateReference = formatPath(path)
	default:
		storeUnknown(el, tag, value)
	}
	return err
}

func decodeMatrixField(el *Element, tag uint64, value ber.TLV) error {
	m := el.Matrix
	var err error
	switch tag {
	case 0:
		el.Identifier, err = value.String()
	case 1:
		el.Description, err = value.String()
	case 2:
		m.Type, err = value.Int64()
	case 3:
		m.AddressingMode, err = value.Int64()
	case 4:
		m.TargetCount, err = value.Int64()
	case 5:
		m.SourceCount, err = value.Int64()
	case 6:
		m.MaximumTotalConnects, err = value.Int64()
	case 7:
		m.MaximumConnectsPerTarget, err = value.Int64()
	case 8:
		if value.Tag.Number == 13 {
			var path []int
			path, err = value.RelativeOID()
			m.ParametersLocation = formatPath(path)
		} else {
			m.ParametersLocation, err = value.Int64()
		}
	case 9:
		m.GainParameterNumber, err = value.Int64()
	case 10:
		m.Labels, err = decodeLabels(value)
	case 11:
		el.SchemaIdentifiers, err = value.String()
	case 12:
		var path []int
		path, err = value.RelativeOID()
		el.TemplateReference = formatPath(path)
	default:
		storeUnknown(el, tag, value)
	}
	return err
}

func decodeFunctionField(el *Element, tag uint64, value ber.TLV) error {
	var err error
	switch tag {
	case 0:
		el.Identifier, err = value.String()
	case 1:
		el.Description, err = value.String()
	case 2:
		el.Function.Arguments, err = decodeTupleDescription(value)
	case 3:
		el.Function.Result, err = decodeTupleDescription(value)
	case 4:
		var path []int
		path, err = value.RelativeOID()
		el.TemplateReference = formatPath(path)
	default:
		storeUnknown(el, tag, value)
	}
	return err
}

func decodeValue(value ber.TLV) (any, error) {
	if value.IsNull() {
		return nil, nil
	}
	if value.Tag.Class != ber.ClassUniversal {
		return nil, errors.New("Glow value is not universally tagged")
	}
	switch value.Tag.Number {
	case 1:
		return value.Bool()
	case 2:
		return value.Int64()
	case 4:
		return append([]byte(nil), value.Content...), nil
	case 9:
		return value.Real()
	case 12:
		return value.String()
	default:
		return nil, fmt.Errorf("unsupported Glow value tag %d", value.Tag.Number)
	}
}

func intValue(value ber.TLV) (int, error) {
	n, err := value.Int64()
	if err != nil {
		return 0, err
	}
	if int64(int(n)) != n {
		return 0, errors.New("Glow integer overflows int")
	}
	return int(n), nil
}

func storeUnknown(el *Element, tag uint64, value ber.TLV) {
	if el.Unknown == nil {
		el.Unknown = make(map[uint64]ber.TLV)
	}
	el.Unknown[tag] = value
}

func finalizeParameterType(el *Element) {
	if el.ElementType != ParameterElement && el.ElementType != QualifiedParameterElement {
		return
	}
	if el.HasValueType {
		return
	}
	if el.ValueType == 5 { // Trigger has precedence over inferred types.
		return
	}
	if el.Enumeration != "" || len(el.EnumMap) > 0 {
		el.ValueType = valueTypeEnum
		return
	}
	if !el.HasValue || el.Value == nil {
		return
	}
	switch el.Value.(type) {
	case int64:
		el.ValueType = valueTypeInt
	case float64:
		el.ValueType = valueTypeReal
	case string:
		el.ValueType = valueTypeString
	case bool:
		el.ValueType = valueTypeBool
	case []byte:
		el.ValueType = 7
	}
}

func decodeEnumMap(value ber.TLV) ([]EnumEntry, error) {
	var out []EnumEntry
	for _, wrapper := range value.Children {
		pair, err := wrapper.Inner()
		if err != nil {
			return nil, err
		}
		nameValue, ok, err := pair.ExplicitChild(0)
		if err != nil || !ok {
			return nil, errors.New("enum entry has no name")
		}
		intValue, ok, err := pair.ExplicitChild(1)
		if err != nil || !ok {
			return nil, errors.New("enum entry has no value")
		}
		name, err := nameValue.String()
		if err != nil {
			return nil, err
		}
		n, err := intValue.Int64()
		if err != nil {
			return nil, err
		}
		out = append(out, EnumEntry{Name: name, Value: n})
	}
	return out, nil
}

func decodeStreamDescriptor(value ber.TLV) (*StreamDescriptor, error) {
	format, ok, err := value.ExplicitChild(0)
	if err != nil || !ok {
		return nil, errors.New("stream descriptor has no format")
	}
	offset, ok, err := value.ExplicitChild(1)
	if err != nil || !ok {
		return nil, errors.New("stream descriptor has no offset")
	}
	f, err := format.Int64()
	if err != nil {
		return nil, err
	}
	o, err := offset.Int64()
	return &StreamDescriptor{Format: f, Offset: o}, err
}

func decodeMatrixCollections(value ber.TLV, matrix *Matrix) error {
	for _, tag := range []uint64{3, 4, 5} {
		collection, ok, err := value.ExplicitChild(tag)
		if err != nil || !ok {
			if err != nil {
				return err
			}
			continue
		}
		switch tag {
		case 3, 4:
			var numbers []int64
			for _, wrapper := range collection.Children {
				signal, err := wrapper.Inner()
				if err != nil {
					return err
				}
				number, ok, err := signal.ExplicitChild(0)
				if err != nil || !ok {
					return errors.New("matrix signal has no number")
				}
				n, err := number.Int64()
				if err != nil {
					return err
				}
				numbers = append(numbers, n)
			}
			if tag == 3 {
				matrix.Targets = numbers
			} else {
				matrix.Sources = numbers
			}
		case 5:
			for _, wrapper := range collection.Children {
				connection, err := wrapper.Inner()
				if err != nil {
					return err
				}
				parsed, err := decodeConnection(connection)
				if err != nil {
					return err
				}
				matrix.Connections = append(matrix.Connections, parsed)
			}
		}
	}
	return nil
}

func decodeConnection(value ber.TLV) (MatrixConnection, error) {
	var out MatrixConnection
	if target, ok, err := value.ExplicitChild(0); err != nil || !ok {
		return out, errors.New("matrix connection has no target")
	} else {
		out.Target, err = target.Int64()
		if err != nil {
			return out, err
		}
	}
	if sources, ok, err := value.ExplicitChild(1); err != nil {
		return out, err
	} else if ok {
		out.Sources, err = sources.RelativeOID()
		if err != nil {
			return out, err
		}
	}
	if operation, ok, err := value.ExplicitChild(2); err != nil {
		return out, err
	} else if ok {
		out.Operation, err = operation.Int64()
		if err != nil {
			return out, err
		}
	}
	if disposition, ok, err := value.ExplicitChild(3); err != nil {
		return out, err
	} else if ok {
		out.Disposition, err = disposition.Int64()
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func decodeLabels(value ber.TLV) ([]Label, error) {
	var out []Label
	for _, wrapper := range value.Children {
		label, err := wrapper.Inner()
		if err != nil {
			return nil, err
		}
		pathValue, ok, err := label.ExplicitChild(0)
		if err != nil || !ok {
			return nil, errors.New("matrix label has no path")
		}
		path, err := pathValue.RelativeOID()
		if err != nil {
			return nil, err
		}
		descriptionValue, ok, err := label.ExplicitChild(1)
		if err != nil || !ok {
			return nil, errors.New("matrix label has no description")
		}
		description, err := descriptionValue.String()
		if err != nil {
			return nil, err
		}
		out = append(out, Label{BasePath: formatPath(path), Description: description})
	}
	return out, nil
}

func decodeTupleDescription(value ber.TLV) ([]TupleItem, error) {
	var out []TupleItem
	for _, wrapper := range value.Children {
		item, err := wrapper.Inner()
		if err != nil {
			return nil, err
		}
		typeValue, ok, err := item.ExplicitChild(0)
		if err != nil || !ok {
			return nil, errors.New("tuple item has no type")
		}
		typeNumber, err := typeValue.Int64()
		if err != nil {
			return nil, err
		}
		parsed := TupleItem{Type: typeNumber}
		if nameValue, ok, err := item.ExplicitChild(1); err != nil {
			return nil, err
		} else if ok {
			parsed.Name, err = nameValue.String()
			if err != nil {
				return nil, err
			}
		}
		out = append(out, parsed)
	}
	return out, nil
}

func decodeCommand(value ber.TLV, el *Element) (*Element, error) {
	number, ok, err := value.ExplicitChild(0)
	if err != nil || !ok {
		return nil, errors.New("Glow command has no number")
	}
	el.Command.Number, err = number.Int64()
	if err != nil {
		return nil, err
	}
	if mask, ok, err := value.ExplicitChild(1); err != nil {
		return nil, err
	} else if ok {
		el.Command.DirFieldMask, err = mask.Int64()
		if err != nil {
			return nil, err
		}
	}
	if invocation, ok, err := value.ExplicitChild(2); err != nil {
		return nil, err
	} else if ok {
		el.Command.Invocation, err = decodeInvocation(invocation)
		if err != nil {
			return nil, err
		}
	}
	return el, nil
}

func decodeInvocation(value ber.TLV) (*Invocation, error) {
	out := &Invocation{}
	if id, ok, err := value.ExplicitChild(0); err != nil {
		return nil, err
	} else if ok {
		out.ID, err = id.Int64()
		if err != nil {
			return nil, err
		}
	}
	if tuple, ok, err := value.ExplicitChild(1); err != nil {
		return nil, err
	} else if ok {
		out.Arguments, err = decodeTuple(tuple)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func decodeInvocationResult(value ber.TLV) (*InvocationResult, error) {
	out := &InvocationResult{Success: true}
	id, ok, err := value.ExplicitChild(0)
	if err != nil || !ok {
		return nil, errors.New("invocation result has no id")
	}
	out.ID, err = id.Int64()
	if err != nil {
		return nil, err
	}
	if success, ok, err := value.ExplicitChild(1); err != nil {
		return nil, err
	} else if ok {
		out.Success, err = success.Bool()
		if err != nil {
			return nil, err
		}
	}
	if tuple, ok, err := value.ExplicitChild(2); err != nil {
		return nil, err
	} else if ok {
		out.Result, err = decodeTuple(tuple)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func decodeTuple(value ber.TLV) ([]any, error) {
	var out []any
	for _, wrapper := range value.Children {
		item, err := wrapper.Inner()
		if err != nil {
			return nil, err
		}
		decoded, err := decodeValue(item)
		if err != nil {
			return nil, err
		}
		out = append(out, decoded)
	}
	return out, nil
}

func decodeTemplate(value ber.TLV, el *Element) (*Element, error) {
	if description, ok, err := value.ExplicitChild(2); err != nil {
		return nil, err
	} else if ok {
		el.Template.Description, err = description.String()
		if err != nil {
			return nil, err
		}
	}
	if templateElement, ok := value.Child(ber.ClassContext, 1); ok {
		inner, err := templateElement.Inner()
		if err != nil {
			return nil, err
		}
		el.Template.Element, err = decodeElement(inner)
		if err != nil {
			return nil, err
		}
	}
	return el, nil
}

func decodeStreams(value ber.TLV) ([]StreamEntry, error) {
	var out []StreamEntry
	for _, wrapper := range value.Children {
		entry, err := wrapper.Inner()
		if err != nil {
			return nil, err
		}
		id, ok, err := entry.ExplicitChild(0)
		if err != nil || !ok {
			return nil, errors.New("stream entry has no identifier")
		}
		identifier, err := id.Int64()
		if err != nil {
			return nil, err
		}
		streamValue, ok, err := entry.ExplicitChild(1)
		if err != nil || !ok {
			return nil, errors.New("stream entry has no value")
		}
		decoded, err := decodeValue(streamValue)
		if err != nil {
			return nil, err
		}
		out = append(out, StreamEntry{Identifier: identifier, Value: decoded})
	}
	return out, nil
}

func formatPath(path []int) string {
	parts := make([]string, len(path))
	for i, component := range path {
		parts[i] = strconv.Itoa(component)
	}
	return strings.Join(parts, ".")
}
