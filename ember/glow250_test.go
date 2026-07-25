package ember

import (
	"testing"

	ber "github.com/johannes-kuhfuss/emberplus/asn1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeSetParameterGlow250RoundTrip(t *testing.T) {
	t.Parallel()

	encoded, err := EncodeSetParameter("1.128.3", -12.5)
	require.NoError(t, err)
	message, err := DecodeRoot(encoded)
	require.NoError(t, err)
	element, err := message.Elements.GetElementByPath("1.128.3")
	require.NoError(t, err)
	assert.Equal(t, QualifiedParameterElement, element.ElementType)
	assert.Equal(t, -12.5, element.Value)
}

func TestEncodeMatrixConnectionsGlow250RoundTrip(t *testing.T) {
	t.Parallel()

	encoded, err := EncodeMatrixConnections("1.200", []MatrixConnection{{Target: 2, Sources: []int{1, 128}, Operation: 1}})
	require.NoError(t, err)
	message, err := DecodeRoot(encoded)
	require.NoError(t, err)
	element, err := message.Elements.GetElementByPath("1.200")
	require.NoError(t, err)
	require.NotNil(t, element.Matrix)
	require.Len(t, element.Matrix.Connections, 1)
	assert.Equal(t, []int{1, 128}, element.Matrix.Connections[0].Sources)
}

func TestDecodeStreamsAndInvocationResult(t *testing.T) {
	t.Parallel()

	id, _ := ber.MarshalExplicit(0, ber.MarshalInteger(7))
	value, _ := ber.MarshalExplicit(1, ber.MarshalBoolean(true))
	entry, _ := ber.MarshalContainer(ber.ClassApplication, 5, id, value)
	wrapper, _ := ber.MarshalExplicit(0, entry)
	streams, _ := ber.MarshalContainer(ber.ClassApplication, 6, wrapper)
	root, _ := ber.MarshalContainer(ber.ClassApplication, 0, streams)
	message, err := DecodeRoot(root)
	require.NoError(t, err)
	assert.Equal(t, []StreamEntry{{Identifier: 7, Value: true}}, message.Streams)

	resultID, _ := ber.MarshalExplicit(0, ber.MarshalInteger(42))
	success, _ := ber.MarshalExplicit(1, ber.MarshalBoolean(false))
	result, _ := ber.MarshalContainer(ber.ClassApplication, 23, resultID, success)
	root, _ = ber.MarshalContainer(ber.ClassApplication, 0, result)
	message, err = DecodeRoot(root)
	require.NoError(t, err)
	require.NotNil(t, message.InvocationResult)
	assert.Equal(t, int64(42), message.InvocationResult.ID)
	assert.False(t, message.InvocationResult.Success)
}

func TestEncodeInvocationContainsCommand(t *testing.T) {
	t.Parallel()

	encoded, err := EncodeInvocation("1.4.1", 51, []any{int64(-1), "x", nil})
	require.NoError(t, err)
	message, err := DecodeRoot(encoded)
	require.NoError(t, err)
	function, err := message.Elements.GetElementByPath("1.4.1")
	require.NoError(t, err)
	require.Len(t, function.Children, 1)
	require.NotNil(t, function.Children[0].Command)
	assert.Equal(t, CommandInvoke, function.Children[0].Command.Number)
	require.NotNil(t, function.Children[0].Command.Invocation)
	assert.Equal(t, []any{int64(-1), "x", nil}, function.Children[0].Command.Invocation.Arguments)
}

func TestDecodeUnqualifiedFunctionAndTemplate(t *testing.T) {
	t.Parallel()

	functionNumber, _ := ber.MarshalExplicit(0, ber.MarshalInteger(4))
	identifier, _ := ber.MarshalExplicit(0, ber.MarshalString("add"))
	functionContents, _ := ber.MarshalContainer(ber.ClassUniversal, 17, identifier)
	functionContentsField, _ := ber.MarshalExplicit(1, functionContents)
	function, _ := ber.MarshalContainer(ber.ClassApplication, 19, functionNumber, functionContentsField)
	functionWrapper, _ := ber.MarshalExplicit(0, function)

	parameterNumber, _ := ber.MarshalExplicit(0, ber.MarshalInteger(1))
	templateParameter, _ := ber.MarshalContainer(ber.ClassApplication, 1, parameterNumber)
	templateNumber, _ := ber.MarshalExplicit(0, ber.MarshalInteger(7))
	templateElement, _ := ber.MarshalExplicit(1, templateParameter)
	templateDescription, _ := ber.MarshalExplicit(2, ber.MarshalString("gain"))
	template, _ := ber.MarshalContainer(ber.ClassApplication, 24, templateNumber, templateElement, templateDescription)
	templateWrapper, _ := ber.MarshalExplicit(0, template)

	collection, _ := ber.MarshalContainer(ber.ClassApplication, 11, functionWrapper, templateWrapper)
	root, _ := ber.MarshalContainer(ber.ClassApplication, 0, collection)
	message, err := DecodeRoot(root)
	require.NoError(t, err)
	decodedFunction, err := message.Elements.GetElementByPath("4")
	require.NoError(t, err)
	assert.Equal(t, FunctionElement, decodedFunction.ElementType)
	assert.Equal(t, "add", decodedFunction.Identifier)
	decodedTemplate, err := message.Elements.GetElementByPath("7")
	require.NoError(t, err)
	require.NotNil(t, decodedTemplate.Template)
	assert.Equal(t, "gain", decodedTemplate.Template.Description)
	require.NotNil(t, decodedTemplate.Template.Element)
	assert.Equal(t, ParameterElement, decodedTemplate.Template.Element.ElementType)
}

func TestParameterSetOrderAndNullableValues(t *testing.T) {
	t.Parallel()

	path, _ := ber.MarshalRelativeOID([]int{2, 3})
	pathField, _ := ber.MarshalExplicit(0, path)
	valueField, _ := ber.MarshalExplicit(2, ber.MarshalInteger(-20))
	defaultField, _ := ber.MarshalExplicit(12, ber.MarshalNull())
	typeField, _ := ber.MarshalExplicit(13, ber.MarshalInteger(1))
	contents, _ := ber.MarshalContainer(ber.ClassUniversal, 17, valueField, defaultField, typeField)
	contentsField, _ := ber.MarshalExplicit(1, contents)
	parameter, _ := ber.MarshalContainer(ber.ClassApplication, 9, pathField, contentsField)
	root, err := encodeRoot(parameter)
	require.NoError(t, err)
	message, err := DecodeRoot(root)
	require.NoError(t, err)
	decoded, err := message.Elements.GetElementByPath("2.3")
	require.NoError(t, err)
	assert.Equal(t, int64(-20), decoded.Value)
	assert.True(t, decoded.HasDefault)
	assert.Nil(t, decoded.Default)
	assert.Equal(t, 1, decoded.ValueType)
	assert.True(t, decoded.HasValueType)
}

func TestExplicitEnumTypeIsNotInferredAsInteger(t *testing.T) {
	t.Parallel()

	path, _ := ber.MarshalRelativeOID([]int{2, 4})
	pathField, _ := ber.MarshalExplicit(0, path)
	valueField, _ := ber.MarshalExplicit(2, ber.MarshalInteger(3))
	typeField, _ := ber.MarshalExplicit(13, ber.MarshalInteger(6))
	contents, _ := ber.MarshalContainer(ber.ClassUniversal, 17, valueField, typeField)
	contentsField, _ := ber.MarshalExplicit(1, contents)
	parameter, _ := ber.MarshalContainer(ber.ClassApplication, 9, pathField, contentsField)
	root, err := encodeRoot(parameter)
	require.NoError(t, err)

	message, err := DecodeRoot(root)
	require.NoError(t, err)
	decoded, err := message.Elements.GetElementByPath("2.4")
	require.NoError(t, err)
	assert.Equal(t, int64(3), decoded.Value)
	assert.Equal(t, 6, decoded.ValueType)
	assert.True(t, decoded.HasValueType)
}

func TestDecodeQualifiedFunctionKeepsQualifiedType(t *testing.T) {
	t.Parallel()

	path, _ := ber.MarshalRelativeOID([]int{1, 2})
	pathField, _ := ber.MarshalExplicit(0, path)
	function, _ := ber.MarshalContainer(ber.ClassApplication, 20, pathField)
	root, err := encodeRoot(function)
	require.NoError(t, err)

	message, err := DecodeRoot(root)
	require.NoError(t, err)
	decoded, err := message.Elements.GetElementByPath("1.2")
	require.NoError(t, err)
	assert.Equal(t, QualifiedFunctionElement, decoded.ElementType)
}
