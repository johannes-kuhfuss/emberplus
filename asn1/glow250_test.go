package asn1

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignedIntegerDecoding(t *testing.T) {
	t.Parallel()

	value, err := NewDecoder([]byte{0x02, 0x01, 0xff}).DecodeInt64()
	require.NoError(t, err)
	assert.Equal(t, int64(-1), value)

	value, err = NewDecoder([]byte{0x02, 0x08, 0x80, 0, 0, 0, 0, 0, 0, 0}).DecodeInt64()
	require.NoError(t, err)
	assert.Equal(t, int64(math.MinInt64), value)
}

func TestRelativeOIDMultiByteRoundTrip(t *testing.T) {
	t.Parallel()

	encoded, err := MarshalRelativeOID([]int{1, 128, 16384})
	require.NoError(t, err)
	assert.Equal(t, []byte{0x0d, 0x06, 0x01, 0x81, 0x00, 0x81, 0x80, 0x00}, encoded)

	decoded, err := NewDecoder(encoded).DecodeUniversal()
	require.NoError(t, err)
	assert.Equal(t, []int{1, 128, 16384}, decoded)
}

func TestRealRoundTrip(t *testing.T) {
	t.Parallel()

	for _, expected := range []float64{0, 1, -12.5, math.SmallestNonzeroFloat64, math.MaxFloat64} {
		encoded, err := MarshalReal(expected)
		require.NoError(t, err)
		actual, err := NewDecoder(encoded).DecodeReal()
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
}

func TestParseIndefiniteTLV(t *testing.T) {
	t.Parallel()

	value, rest, err := ParseTLV([]byte{0x60, 0x80, 0x02, 0x01, 0xff, 0x00, 0x00, 0xaa})
	require.NoError(t, err)
	assert.Equal(t, []byte{0xaa}, rest)
	require.Len(t, value.Children, 1)
	integer, err := value.Children[0].Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(-1), integer)
}
