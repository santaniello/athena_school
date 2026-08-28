package knowledge

import "errors"

// Match type values a DuplicateMatch can carry.
const (
	MatchExact    = "exact"
	MatchSemantic = "semantic"
)

// Defaults for FindDuplicates when a caller does not override them.
const (
	DefaultDuplicateTopK       = 5
	DefaultDuplicateSimilarity = 0.90
)

// DuplicateMatch is one existing Item a candidate concept might already be
// represented by. See specs/phases/phase-02-knowledge-engine/10-duplicate-detection.md.
type DuplicateMatch struct {
	ItemID    string
	Concept   string
	Status    string
	MatchType string
	Score     float64
}

// ErrSemanticDuplicateCheckUnavailable is returned when the semantic stage
// of duplicate detection could not run to completion — an embedding call or
// a VectorStore search failed. The exact-match results already computed are
// still returned alongside this error; the candidate stays usable, only the
// semantic check is reported as incomplete. See
// specs/phases/phase-02-knowledge-engine/10-01-duplicate-detection-decisions.md
// Decision 2.
var ErrSemanticDuplicateCheckUnavailable = errors.New("semantic duplicate check unavailable")
