package knowledge

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateString_trimsSurroundingWhitespace(t *testing.T) {
	// Given a value padded with whitespace
	// When truncating it well within the limit
	got := truncateString("  Idempotency key \n", 50)

	// Then only the padding is removed
	assert.Equal(t, "Idempotency key", got)
}

func TestTruncateString_keepsAValueExactlyAtTheLimit(t *testing.T) {
	// Given a value with exactly the maximum number of characters
	value := strings.Repeat("a", 5)

	// When truncating it to that maximum
	got := truncateString(value, 5)

	// Then nothing is cut
	assert.Equal(t, value, got)
}

func TestTruncateString_cutsAnOverlongValueByCharactersNotBytes(t *testing.T) {
	// Given a multi-byte value one character over the limit
	value := strings.Repeat("é", 6)

	// When truncating it to five characters
	got := truncateString(value, 5)

	// Then exactly five characters remain, none split in half
	assert.Equal(t, strings.Repeat("é", 5), got)
}

func TestTruncateString_returnsEmptyForAWhitespaceOnlyValue(t *testing.T) {
	// Given a value with nothing but whitespace
	// When truncating it
	got := truncateString("   ", 5)

	// Then nothing is left
	assert.Empty(t, got)
}

func TestNormalizeList_returnsAnEmptyNonNilListForNoValues(t *testing.T) {
	// Given no values at all
	// When normalizing them
	got := normalizeList(nil)

	// Then the result is an empty list, never nil
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestNormalizeList_trimsEntriesAndDropsBlankOnes(t *testing.T) {
	// Given entries with padding and blanks between them
	// When normalizing them
	got := normalizeList([]string{"  first ", "", "   ", "second"})

	// Then blanks are dropped and the rest are trimmed, in order
	assert.Equal(t, []string{"first", "second"}, got)
}

func TestNormalizeList_truncatesEachEntryToTheEntryLimit(t *testing.T) {
	// Given an entry one character over the per-entry limit
	long := strings.Repeat("x", maxListEntryChars+1)

	// When normalizing it
	got := normalizeList([]string{long})

	// Then it is cut to exactly the limit
	assert.Equal(t, []string{strings.Repeat("x", maxListEntryChars)}, got)
}

func TestNormalizeList_keepsAtMostTheListLimit_countingOnlyKeptEntries(t *testing.T) {
	// Given the limit plus two entries, with two blanks up front that must
	// not count toward it
	values := []string{"", " "}
	for i := 0; i < maxListItems+2; i++ {
		values = append(values, "entry-"+strconv.Itoa(i))
	}

	// When normalizing them
	got := normalizeList(values)

	// Then exactly the first maxListItems real entries survive
	assert.Len(t, got, maxListItems)
	assert.Equal(t, "entry-0", got[0])
	assert.Equal(t, "entry-"+strconv.Itoa(maxListItems-1), got[maxListItems-1])
}

func TestNormalizeList_keepsEveryEntryWhenExactlyAtTheListLimit(t *testing.T) {
	// Given exactly the limit's worth of entries
	values := make([]string, maxListItems)
	for i := range values {
		values[i] = "entry-" + strconv.Itoa(i)
	}

	// When normalizing them
	got := normalizeList(values)

	// Then none are dropped
	assert.Equal(t, values, got)
}
