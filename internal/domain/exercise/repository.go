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
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
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
