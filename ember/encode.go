package ember

import (
	"fmt"

	ber "github.com/johannes-kuhfuss/emberplus/asn1"
)

const (
	CommandSubscribe    int64 = 30
	CommandUnsubscribe  int64 = 31
	CommandGetDirectory int64 = 32
	CommandInvoke       int64 = 33
)

// EncodeGetDirectory encodes a Glow GetDirectory request. An empty path queries the root.
func EncodeGetDirectory(elementType ElementType, path string, fieldMask int64) ([]byte, error) {
	command, err := encodeCommand(CommandGetDirectory, fieldMask, nil)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return encodeRoot(command)
	}
	return encodeElementCommand(elementType, path, command)
}

// EncodeSubscription encodes a Subscribe or Unsubscribe request.
func EncodeSubscription(elementType ElementType, path string, subscribe bool) ([]byte, error) {
	number := CommandUnsubscribe
	if subscribe {
		number = CommandSubscribe
	}
	command, err := encodeCommand(number, 0, nil)
	if err != nil {
		return nil, err
	}
	return encodeElementCommand(elementType, path, command)
}

// EncodeSetParameter encodes a qualified parameter value change.
func EncodeSetParameter(path string, value any) ([]byte, error) {
	parsed, err := parsePath(path)
	if err != nil || len(parsed) == 0 {
		return nil, fmt.Errorf("invalid parameter path %q", path)
	}
	oid, err := ber.MarshalRelativeOID(parsed)
	if err != nil {
		return nil, err
	}
	pathField, _ := ber.MarshalExplicit(0, oid)
	encodedValue, err := MarshalValue(value)
	if err != nil {
		return nil, err
	}
	valueField, _ := ber.MarshalExplicit(2, encodedValue)
	contents, _ := ber.MarshalContainer(ber.ClassUniversal, 17, valueField)
	contentsField, _ := ber.MarshalExplicit(1, contents)
	parameter, _ := ber.MarshalContainer(ber.ClassApplication, 9, pathField, contentsField)
	return encodeRoot(parameter)
}

// EncodeMatrixConnections encodes qualified matrix connection changes.
func EncodeMatrixConnections(path string, connections []MatrixConnection) ([]byte, error) {
	parsed, err := parsePath(path)
	if err != nil || len(parsed) == 0 {
		return nil, fmt.Errorf("invalid matrix path %q", path)
	}
	oid, err := ber.MarshalRelativeOID(parsed)
	if err != nil {
		return nil, err
	}
	pathField, _ := ber.MarshalExplicit(0, oid)
	var entries [][]byte
	for _, connection := range connections {
		fields := make([][]byte, 0, 4)
		target, _ := ber.MarshalExplicit(0, ber.MarshalInteger(connection.Target))
		fields = append(fields, target)
		if connection.Sources != nil {
			sources, err := ber.MarshalRelativeOID(connection.Sources)
			if err != nil {
				return nil, err
			}
			sourcesField, _ := ber.MarshalExplicit(1, sources)
			fields = append(fields, sourcesField)
		}
		operation, _ := ber.MarshalExplicit(2, ber.MarshalInteger(connection.Operation))
		fields = append(fields, operation)
		encoded, _ := ber.MarshalContainer(ber.ClassApplication, 16, fields...)
		wrapper, _ := ber.MarshalExplicit(0, encoded)
		entries = append(entries, wrapper)
	}
	collection, _ := ber.MarshalContainer(ber.ClassUniversal, 16, entries...)
	connectionsField, _ := ber.MarshalExplicit(5, collection)
	matrix, _ := ber.MarshalContainer(ber.ClassApplication, 17, pathField, connectionsField)
	return encodeRoot(matrix)
}

// EncodeInvocation encodes a qualified function invocation.
func EncodeInvocation(path string, id int64, arguments []any) ([]byte, error) {
	fields := make([][]byte, 0, 2)
	if id != 0 {
		idField, _ := ber.MarshalExplicit(0, ber.MarshalInteger(id))
		fields = append(fields, idField)
	}
	if arguments != nil {
		var values [][]byte
		for _, argument := range arguments {
			value, err := MarshalValue(argument)
			if err != nil {
				return nil, err
			}
			wrapper, _ := ber.MarshalExplicit(0, value)
			values = append(values, wrapper)
		}
		tuple, _ := ber.MarshalContainer(ber.ClassUniversal, 16, values...)
		argumentsField, _ := ber.MarshalExplicit(1, tuple)
		fields = append(fields, argumentsField)
	}
	invocation, _ := ber.MarshalContainer(ber.ClassApplication, 22, fields...)
	command, err := encodeCommand(CommandInvoke, 0, invocation)
	if err != nil {
		return nil, err
	}
	return encodeElementCommand(FunctionElement, path, command)
}

// MarshalValue encodes any Glow 2.50 Value alternative. A nil value becomes NULL.
func MarshalValue(value any) ([]byte, error) {
	switch v := value.(type) {
	case nil:
		return ber.MarshalNull(), nil
	case bool:
		return ber.MarshalBoolean(v), nil
	case string:
		return ber.MarshalString(v), nil
	case []byte:
		return ber.MarshalOctets(v), nil
	case float32:
		return ber.MarshalReal(float64(v))
	case float64:
		return ber.MarshalReal(v)
	case int:
		return ber.MarshalInteger(int64(v)), nil
	case int8:
		return ber.MarshalInteger(int64(v)), nil
	case int16:
		return ber.MarshalInteger(int64(v)), nil
	case int32:
		return ber.MarshalInteger(int64(v)), nil
	case int64:
		return ber.MarshalInteger(v), nil
	case uint:
		if uint64(v) > uint64(^uint64(0)>>1) {
			return nil, fmt.Errorf("Glow INTEGER overflows int64: %d", v)
		}
		return ber.MarshalInteger(int64(v)), nil
	case uint8:
		return ber.MarshalInteger(int64(v)), nil
	case uint16:
		return ber.MarshalInteger(int64(v)), nil
	case uint32:
		return ber.MarshalInteger(int64(v)), nil
	case uint64:
		if v > uint64(^uint64(0)>>1) {
			return nil, fmt.Errorf("Glow INTEGER overflows int64: %d", v)
		}
		return ber.MarshalInteger(int64(v)), nil
	default:
		return nil, fmt.Errorf("unsupported Glow value type %T", value)
	}
}

func encodeElementCommand(elementType ElementType, path string, command []byte) ([]byte, error) {
	parsed, err := parsePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path %q: %w", path, err)
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("invalid element path %q", path)
	}
	oid, err := ber.MarshalRelativeOID(parsed)
	if err != nil {
		return nil, err
	}
	pathField, _ := ber.MarshalExplicit(0, oid)
	commandWrapper, _ := ber.MarshalExplicit(0, command)
	children, _ := ber.MarshalContainer(ber.ClassApplication, 4, commandWrapper)
	childrenField, _ := ber.MarshalExplicit(2, children)
	tag, err := qualifiedTag(elementType)
	if err != nil {
		return nil, err
	}
	element, _ := ber.MarshalContainer(ber.ClassApplication, tag, pathField, childrenField)
	return encodeRoot(element)
}

func encodeCommand(number, fieldMask int64, invocation []byte) ([]byte, error) {
	numberField, _ := ber.MarshalExplicit(0, ber.MarshalInteger(number))
	fields := [][]byte{numberField}
	if number == CommandGetDirectory {
		maskField, _ := ber.MarshalExplicit(1, ber.MarshalInteger(fieldMask))
		fields = append(fields, maskField)
	}
	if number == CommandInvoke {
		if invocation == nil {
			return nil, fmt.Errorf("invoke command requires invocation data")
		}
		invocationField, _ := ber.MarshalExplicit(2, invocation)
		fields = append(fields, invocationField)
	}
	return ber.MarshalContainer(ber.ClassApplication, 2, fields...)
}

func encodeRoot(elements ...[]byte) ([]byte, error) {
	var entries [][]byte
	for _, element := range elements {
		wrapper, _ := ber.MarshalExplicit(0, element)
		entries = append(entries, wrapper)
	}
	collection, _ := ber.MarshalContainer(ber.ClassApplication, 11, entries...)
	return ber.MarshalContainer(ber.ClassApplication, 0, collection)
}

func qualifiedTag(elementType ElementType) (uint64, error) {
	switch elementType {
	case ParameterElement, QualifiedParameterElement:
		return 9, nil
	case NodeElement, QualifiedNodeElement:
		return 10, nil
	case MatrixElement, QualifiedMatrixElement:
		return 17, nil
	case FunctionElement, QualifiedFunctionElement:
		return 20, nil
	case TemplateElement, QualifiedTemplateElement:
		return 25, nil
	default:
		return 0, fmt.Errorf("unsupported Glow element type %q", elementType)
	}
}
