package knowledge

import "errors"

// ErrMalformedExtraction is returned when an LLM response has no valid JSON envelope.
var ErrMalformedExtraction = errors.New("malformed knowledge extraction response")

// ErrTranscriptTooLarge is returned when no complete transcript message fits
// within the extraction budget.
var ErrTranscriptTooLarge = errors.New("no complete transcript message fits within the extraction limit")
