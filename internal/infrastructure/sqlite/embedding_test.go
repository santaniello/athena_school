package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeEmbedding_matchesHandWrittenLittleEndianBytes(t *testing.T) {
	// Given a two-component vector with recognizable values
	vec := []float32{1.0, -2.0}

	// When encoding it
	blob := encodeEmbedding(vec)

	// Then the bytes match a hand-written little-endian IEEE-754 encoding:
	// 1.0 = 0x3F800000, -2.0 = 0xC0000000
	assert.Equal(t, []byte{
		0x00, 0x00, 0x80, 0x3F,
		0x00, 0x00, 0x00, 0xC0,
	}, blob)
}

func TestEncodeThenDecodeEmbedding_roundTrips(t *testing.T) {
	// Given a vector with several distinct values
	vec := []float32{0.1, -0.25, 3.5, 0, -1234.5678}

	// When encoding then decoding it
	decoded, err := decodeEmbedding(encodeEmbedding(vec))

	// Then the original values are recovered exactly
	require.NoError(t, err)
	assert.Equal(t, vec, decoded)
}

func TestDecodeEmbedding_returnsError_whenBlobLengthIsOdd(t *testing.T) {
	// Given a blob whose length (5) is not a multiple of 4
	blob := make([]byte, 5)

	// When decoding it
	_, err := decodeEmbedding(blob)

	// Then it fails
	assert.Error(t, err)
}

func TestDecodeEmbedding_returnsError_whenBlobLengthIsEvenButNotAMultipleOfFour(t *testing.T) {
	// Given a blob whose length (6) is even but still not a multiple of 4
	blob := make([]byte, 6)

	// When decoding it
	_, err := decodeEmbedding(blob)

	// Then it fails — the rule is len%4, not parity
	assert.Error(t, err)
}

func TestDecodeEmbedding_returnsEmptySlice_forEmptyBlob(t *testing.T) {
	// Given an empty blob
	// When decoding it
	decoded, err := decodeEmbedding([]byte{})

	// Then it succeeds with zero components
	require.NoError(t, err)
	assert.Empty(t, decoded)
}
