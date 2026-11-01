package exercise

import (
	"context"
	"errors"
	"strings"
)

// ErrChallengeNotFound is returned when no challenge matches the given ID.
var ErrChallengeNotFound = errors.New("challenge not found")

// Listing page bounds.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 200
)

// NormalizeTopic is the form a challenge's topic snapshot is stored and
// filtered in: trimmed and lower-cased.
func NormalizeTopic(topic string) string {
	return strings.ToLower(strings.TrimSpace(topic))
}

// Page selects one slice of a listing.
type Page struct {
	Limit  int
	Offset int
}

// Normalize returns the page with Limit within (0, MaxPageLimit] and a
// non-negative Offset. A missing Limit becomes DefaultPageLimit.
func (p Page) Normalize() Page {
	if p.Limit <= 0 {
		p.Limit = DefaultPageLimit
	}
	return Page{Limit: min(p.Limit, MaxPageLimit), Offset: max(p.Offset, 0)}
}

// ChallengeFilter narrows a listing. Every non-empty field must match (AND).
type ChallengeFilter struct {
	Query      string // case-insensitive over title, scenario and topic
	Topic      string // exact, on the normalized snapshot
	Difficulty Difficulty
	Origin     ChallengeOrigin
}

// ChallengePage is one slice of a listing plus the total matching count.
type ChallengePage struct {
	Items []Challenge
	Total int
}

// ChallengeRepository persists Challenges. Ordering of List is UpdatedAt
// descending, then ID, so pages are stable.
type ChallengeRepository interface {
	Create(ctx context.Context, challenge Challenge) error
	// Get returns the challenge, or ErrChallengeNotFound.
	Get(ctx context.Context, id string) (Challenge, error)
	List(ctx context.Context, filter ChallengeFilter, page Page) (ChallengePage, error)
}

// ErrRubricNotFound is returned when no rubric version matches, or a
// challenge has no rubric yet.
var ErrRubricNotFound = errors.New("rubric not found")

// ErrRubricVersionConflict is returned by RubricRepository.Create when the
// challenge already has a rubric with that version number, typically because
// another call stored it first. Retrying with a fresh Latest resolves it.
var ErrRubricVersionConflict = errors.New("rubric version already exists for this challenge")

// RubricRepository persists RubricVersions. A stored version is immutable, so
// there is deliberately no Update and no Delete.
type RubricRepository interface {
	// Create stores the version and its criteria atomically. It returns
	// ErrRubricVersionConflict if the challenge already has that version number.
	Create(ctx context.Context, version RubricVersion) error
	// Get returns the version, or ErrRubricNotFound.
	Get(ctx context.Context, versionID string) (RubricVersion, error)
	// Latest returns the challenge's highest version, or ErrRubricNotFound
	// if it has none.
	Latest(ctx context.Context, challengeID string) (RubricVersion, error)
	// ListVersions returns every version of the challenge's rubric, oldest
	// first. A challenge without a rubric yields an empty list.
	ListVersions(ctx context.Context, challengeID string) ([]RubricVersion, error)
}
