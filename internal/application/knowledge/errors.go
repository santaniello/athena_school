package knowledge

import "errors"

// ErrMalformedExtraction is returned when an LLM response has no valid JSON envelope.
var ErrMalformedExtraction = errors.New("malformed knowledge extraction response")

// ErrTranscriptTooLarge is returned when no complete transcript message fits
// within the extraction budget.
var ErrTranscriptTooLarge = errors.New("no complete transcript message fits within the extraction limit")

// ErrIndexLoading is returned by IndexLoader.CheckMutationAllowed while an
// initial load or a retry is in progress, so a concurrent knowledge
// mutation can never race a snapshot publish.
var ErrIndexLoading = errors.New("knowledge index is loading")
