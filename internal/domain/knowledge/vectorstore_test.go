package knowledge

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func validChunk() Chunk {
	return Chunk{
		ID:        "chunk-1",
		Source:    SourceImportedDoc,
		Topic:     "Go",
		ItemID:    "item-1",
		Embedding: []float32{1, 2, 3},
	}
}

func TestValidateChunk_succeeds_withValidChunk(t *testing.T) {
	// Given a structurally valid chunk with a finite, non-zero-norm embedding
	chunk := validChunk()

	// When validating it
	err := ValidateChunk(chunk)

	// Then it passes
	assert.NoError(t, err)
}

func TestValidateChunk_returnsErrInvalidChunkID_whenIDIsEmpty(t *testing.T) {
	// Given a chunk with an empty ID
	chunk := validChunk()
	chunk.ID = ""

	// When validating it
	err := ValidateChunk(chunk)

	// Then it is rejected as an invalid chunk ID
	assert.ErrorIs(t, err, ErrInvalidChunkID)
}

func TestValidateChunk_returnsErrInvalidChunkID_whenIDIsWhitespaceOnly(t *testing.T) {
	// Given a chunk whose ID is only whitespace
	chunk := validChunk()
	chunk.ID = "   "

	// When validating it
	err := ValidateChunk(chunk)

	// Then it is rejected — IDs are validated, never trimmed or rewritten
	assert.ErrorIs(t, err, ErrInvalidChunkID)
}

func TestValidateChunk_returnsErrUnknownSource_whenSourceIsNotAnImportedDocument(t *testing.T) {
	// Given chunks whose source is unrecognized, or one of the sources that
	// only conversation extraction used to produce
	for _, source := range []string{"unknown_source", "athena", "user_note", ""} {
		chunk := validChunk()
		chunk.Source = source

		// When validating them
		err := ValidateChunk(chunk)

		// Then each is rejected as an unknown source
		assert.ErrorIs(t, err, ErrUnknownSource, "source %q", source)
	}
}

func TestValidateChunk_returnsErrInvalidVector_whenEmbeddingIsEmpty(t *testing.T) {
	// Given a chunk with no embedding at all
	chunk := validChunk()
	chunk.Embedding = nil

	// When validating it
	err := ValidateChunk(chunk)

	// Then it is rejected as an invalid vector
	assert.ErrorIs(t, err, ErrInvalidVector)
}

func TestValidateChunk_returnsErrInvalidVector_whenEmbeddingContainsNaN(t *testing.T) {
	// Given a chunk whose embedding contains NaN
	chunk := validChunk()
	chunk.Embedding = []float32{1, float32(math.NaN()), 3}

	// When validating it
	err := ValidateChunk(chunk)

	// Then it is rejected as an invalid vector
	assert.ErrorIs(t, err, ErrInvalidVector)
}

func TestValidateChunk_returnsErrInvalidVector_whenEmbeddingContainsPositiveInfinity(t *testing.T) {
	// Given a chunk whose embedding contains +Inf
	chunk := validChunk()
	chunk.Embedding = []float32{1, float32(math.Inf(1)), 3}

	// When validating it
	err := ValidateChunk(chunk)

	// Then it is rejected as an invalid vector
	assert.ErrorIs(t, err, ErrInvalidVector)
}

func TestValidateChunk_returnsErrInvalidVector_whenEmbeddingContainsNegativeInfinity(t *testing.T) {
	// Given a chunk whose embedding contains -Inf
	chunk := validChunk()
	chunk.Embedding = []float32{1, float32(math.Inf(-1)), 3}

	// When validating it
	err := ValidateChunk(chunk)

	// Then it is rejected as an invalid vector
	assert.ErrorIs(t, err, ErrInvalidVector)
}

func TestValidateChunk_returnsErrInvalidVector_whenNormIsZero(t *testing.T) {
	// Given a chunk whose embedding is all zeros
	chunk := validChunk()
	chunk.Embedding = []float32{0, 0, 0}

	// When validating it
	err := ValidateChunk(chunk)

	// Then it is rejected as an invalid vector, not divided by a zero norm
	assert.ErrorIs(t, err, ErrInvalidVector)
}

func TestValidateChunk_prefersStructuralErrorOverVectorError(t *testing.T) {
	// Given a chunk that violates both a structural rule and the vector rule
	chunk := validChunk()
	chunk.ID = ""
	chunk.Embedding = nil

	// When validating it
	err := ValidateChunk(chunk)

	// Then the structural error takes precedence over the vector error
	assert.ErrorIs(t, err, ErrInvalidChunkID)
	assert.NotErrorIs(t, err, ErrInvalidVector)
}

func TestValidateVector_succeeds_withFiniteNonZeroNormVector(t *testing.T) {
	// Given a finite, non-zero-norm vector
	vec := []float32{3, 4}

	// When validating it
	err := ValidateVector(vec)

	// Then it passes
	assert.NoError(t, err)
}

func TestValidateVector_returnsErrInvalidVector_whenEmpty(t *testing.T) {
	// Given an empty vector
	vec := []float32{}

	// When validating it
	err := ValidateVector(vec)

	// Then it is rejected
	assert.ErrorIs(t, err, ErrInvalidVector)
}

func TestValidateVector_returnsErrInvalidVector_whenNormIsZero(t *testing.T) {
	// Given a vector whose components are all zero
	vec := []float32{0, 0, 0, 0}

	// When validating it
	err := ValidateVector(vec)

	// Then it is rejected rather than divided by a zero norm downstream
	assert.ErrorIs(t, err, ErrInvalidVector)
}

func TestReasonForValidationError_mapsEachValidateChunkSentinel_toItsStableCode(t *testing.T) {
	cases := map[error]string{
		ErrInvalidChunkID: ChunkIssueInvalidChunkID,
		ErrUnknownSource:  ChunkIssueUnknownSource,
		ErrInvalidVector:  ChunkIssueInvalidVector,
	}
	for err, want := range cases {
		// Given each error ValidateChunk can return
		// When mapping it to a stable ChunkLoadIssue reason code
		got := ReasonForValidationError(err)

		// Then it maps to the matching reason code
		assert.Equal(t, want, got, "for %v", err)
	}
}

func TestValidateVector_doesNotOverflow_withManyLargeComponents(t *testing.T) {
	// Given a high-dimensional vector whose squared components would
	// overflow float32 accumulation (float32 max is ~3.4e38; 2000 dims of
	// 1e19 squared each sum past that if accumulated in float32)
	vec := make([]float32, 2000)
	for i := range vec {
		vec[i] = 1e19
	}

	// When validating it
	err := ValidateVector(vec)

	// Then the float64 accumulator keeps the norm finite and valid
	assert.NoError(t, err)
}
