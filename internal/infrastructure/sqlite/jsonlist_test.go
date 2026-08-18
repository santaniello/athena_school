package sqlite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalStringList_encodesNilAsEmptyJSONArray(t *testing.T) {
	// Given a nil slice

	// When marshaling it
	encoded := marshalStringList(nil)

	// Then it encodes as an empty JSON array, never NULL
	assert.Equal(t, "[]", encoded)
}

func TestMarshalStringList_encodesEmptySliceAsEmptyJSONArray(t *testing.T) {
	// Given an empty slice
	empty := []string{}

	// When marshaling it
	encoded := marshalStringList(empty)

	// Then it encodes as an empty JSON array
	assert.Equal(t, "[]", encoded)
}

func TestMarshalStringList_roundTripsThreeElements(t *testing.T) {
	// Given a three-element slice
	values := []string{"one", "two", "three"}

	// When marshaling and unmarshaling it back
	encoded := marshalStringList(values)
	decoded, err := unmarshalStringList(encoded)

	// Then it round-trips exactly
	require.NoError(t, err)
	assert.Equal(t, values, decoded)
}

func TestUnmarshalStringList_decodesEmptyStringAsEmptySlice(t *testing.T) {
	// Given an empty string (legacy/NULL-coalesced column value)

	// When decoding it
	decoded, err := unmarshalStringList("")

	// Then it decodes as an empty slice, never nil
	require.NoError(t, err)
	assert.Equal(t, []string{}, decoded)
}

func TestUnmarshalStringList_returnsError_forInvalidJSON(t *testing.T) {
	// Given a column value that isn't valid JSON

	// When decoding it
	_, err := unmarshalStringList("not json")

	// Then it fails instead of silently returning an empty slice
	assert.Error(t, err)
}
