package study

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripCitationMarkers_removesSingleMarker(t *testing.T) {
	// Given a reply with one citation marker
	content := "Channels are typed pipes [1]."

	// When stripping citation markers
	result := stripCitationMarkers(content)

	// Then the marker is gone, the rest of the text untouched
	assert.Equal(t, "Channels are typed pipes .", result)
}

func TestStripCitationMarkers_removesMultipleAndMultiDigitMarkers(t *testing.T) {
	// Given a reply citing several passages, one with a multi-digit index
	content := "Use select [1] to multiplex [2] across [12] channels."

	// When stripping citation markers
	result := stripCitationMarkers(content)

	// Then every marker is removed
	assert.Equal(t, "Use select  to multiplex  across  channels.", result)
}

func TestStripCitationMarkers_leavesContentWithoutMarkersUnchanged(t *testing.T) {
	// Given a reply with no citation markers
	content := "Channels are typed pipes."

	// When stripping citation markers
	result := stripCitationMarkers(content)

	// Then the content is returned verbatim
	assert.Equal(t, content, result)
}

func TestStripCitationMarkers_handlesEmptyString(t *testing.T) {
	// Given an empty string
	// When stripping citation markers
	result := stripCitationMarkers("")

	// Then it stays empty
	assert.Equal(t, "", result)
}

func TestStripCitationMarkers_leavesBracketedNonDigitWordAlone(t *testing.T) {
	// Given a bracketed word that isn't a citation marker
	content := "See the [todo] before shipping."

	// When stripping citation markers
	result := stripCitationMarkers(content)

	// Then it is left untouched
	assert.Equal(t, content, result)
}
