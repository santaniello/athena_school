package study

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// ProgressConcept is one concept in a session's progress, with the proofs
// behind it, the newest first.
type ProgressConcept struct {
	Name            string
	Status          ConceptStatus
	AssistanceCount int
	EvidenceCount   int
	UpdatedAt       time.Time
	Evidence        []Evidence
}

// SessionProgress is what the tutor has observed in one session, grouped for
// the user: what they demonstrated apart from what was only explained. It is
// the tutor's estimate, not a verdict.
type SessionProgress struct {
	Demonstrated  []ProgressConcept // status DEMONSTRATED
	ExplainedOnly []ProgressConcept // status INTRODUCED: explained, not yet shown by the user
	Difficulties  []ProgressConcept // status PARTIALLY_UNDERSTOOD
	NeedsReview   []ProgressConcept // status NEEDS_REVIEW
}

// IsEmpty reports whether no concept was observed.
func (p SessionProgress) IsEmpty() bool {
	return len(p.Demonstrated) == 0 && len(p.ExplainedOnly) == 0 && len(p.Difficulties) == 0 && len(p.NeedsReview) == 0
}

// BuildSessionProgress groups the session's concepts by status, most recently
// updated first (ties by name), and attaches each one's proofs, newest first.
// A proof whose concept is not in concepts is dropped. Every group is a
// non-nil slice.
func BuildSessionProgress(concepts []ConceptState, evidence []Evidence) SessionProgress {
	proofs := make(map[string][]Evidence)
	for _, proof := range evidence {
		proofs[proof.Concept] = append(proofs[proof.Concept], proof)
	}

	progress := SessionProgress{
		Demonstrated:  []ProgressConcept{},
		ExplainedOnly: []ProgressConcept{},
		Difficulties:  []ProgressConcept{},
		NeedsReview:   []ProgressConcept{},
	}
	groups := map[ConceptStatus]*[]ProgressConcept{
		StatusDemonstrated:        &progress.Demonstrated,
		StatusIntroduced:          &progress.ExplainedOnly,
		StatusPartiallyUnderstood: &progress.Difficulties,
		StatusNeedsReview:         &progress.NeedsReview,
	}
	for _, concept := range concepts {
		group, ok := groups[concept.Status]
		if !ok {
			continue
		}
		*group = append(*group, ProgressConcept{
			Name:            concept.Name,
			Status:          concept.Status,
			AssistanceCount: concept.AssistanceCount,
			EvidenceCount:   concept.EvidenceCount,
			UpdatedAt:       concept.UpdatedAt,
			Evidence:        proofsNewestFirst(proofs[concept.Name]),
		})
	}
	for _, group := range groups {
		sortByRecentUpdate(*group)
	}
	return progress
}

func sortByRecentUpdate(concepts []ProgressConcept) {
	slices.SortStableFunc(concepts, func(a, b ProgressConcept) int {
		return cmp.Or(b.UpdatedAt.Compare(a.UpdatedAt), strings.Compare(a.Name, b.Name))
	})
}

func proofsNewestFirst(proofs []Evidence) []Evidence {
	sorted := slices.Clone(proofs)
	slices.SortStableFunc(sorted, func(a, b Evidence) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return sorted
}
