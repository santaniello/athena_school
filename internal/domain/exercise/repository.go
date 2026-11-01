package exercise

import (
	"context"
	"errors"
)

// ErrChallengeNotFound is returned when no challenge matches the given ID.
var ErrChallengeNotFound = errors.New("challenge not found")

// Page selects one slice of a listing.
type Page struct {
	Limit  int
	Offset int
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
