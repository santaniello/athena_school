package knowledge

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRetrievalThresholds_succeeds_withDefaults(t *testing.T) {
	// Given the documented default thresholds
	// When constructing RetrievalThresholds from them
	got, err := NewRetrievalThresholds(DefaultMinSimilarity, DefaultSufficiency)

	// Then it succeeds and preserves both values
	assert.NoError(t, err)
	assert.Equal(t, DefaultMinSimilarity, got.MinSimilarity)
	assert.Equal(t, DefaultSufficiency, got.Sufficiency)
}

func TestNewRetrievalThresholds_succeeds_whenMinScoreEqualsSufficiency(t *testing.T) {
	// Given minScore and sufficiencyScore set to the same value
	// When constructing RetrievalThresholds
	_, err := NewRetrievalThresholds(0.5, 0.5)

	// Then equality is allowed
	assert.NoError(t, err)
}

func TestNewRetrievalThresholds_succeeds_atCosineRangeBoundaries(t *testing.T) {
	// Given the extreme ends of the valid cosine range
	// When constructing RetrievalThresholds
	_, err := NewRetrievalThresholds(-1, 1)

	// Then the boundaries themselves are accepted
	assert.NoError(t, err)
}

func TestNewRetrievalThresholds_returnsErrInvalidThresholds_whenMinScoreIsNaN(t *testing.T) {
	// Given a non-finite minScore
	// When constructing RetrievalThresholds
	_, err := NewRetrievalThresholds(math.NaN(), 0.5)

	// Then it is rejected
	assert.ErrorIs(t, err, ErrInvalidThresholds)
}

func TestNewRetrievalThresholds_returnsErrInvalidThresholds_whenSufficiencyIsPositiveInfinity(t *testing.T) {
	// Given a non-finite sufficiencyScore
	// When constructing RetrievalThresholds
	_, err := NewRetrievalThresholds(0.3, math.Inf(1))

	// Then it is rejected
	assert.ErrorIs(t, err, ErrInvalidThresholds)
}

func TestNewRetrievalThresholds_returnsErrInvalidThresholds_whenSufficiencyIsNegativeInfinity(t *testing.T) {
	// Given a non-finite sufficiencyScore
	// When constructing RetrievalThresholds
	_, err := NewRetrievalThresholds(0.3, math.Inf(-1))

	// Then it is rejected
	assert.ErrorIs(t, err, ErrInvalidThresholds)
}

func TestNewRetrievalThresholds_returnsErrInvalidThresholds_whenMinScoreBelowNegativeOne(t *testing.T) {
	// Given a minScore outside the cosine range
	// When constructing RetrievalThresholds
	_, err := NewRetrievalThresholds(-1.01, 0.5)

	// Then it is rejected
	assert.ErrorIs(t, err, ErrInvalidThresholds)
}

func TestNewRetrievalThresholds_returnsErrInvalidThresholds_whenSufficiencyAboveOne(t *testing.T) {
	// Given a sufficiencyScore outside the cosine range
	// When constructing RetrievalThresholds
	_, err := NewRetrievalThresholds(0.3, 1.01)

	// Then it is rejected
	assert.ErrorIs(t, err, ErrInvalidThresholds)
}

func TestNewRetrievalThresholds_returnsErrInvalidThresholds_whenMinScoreGreaterThanSufficiency(t *testing.T) {
	// Given a minScore above sufficiencyScore
	// When constructing RetrievalThresholds
	_, err := NewRetrievalThresholds(0.6, 0.55)

	// Then it is rejected — minScore must never exceed sufficiencyScore
	assert.ErrorIs(t, err, ErrInvalidThresholds)
}
