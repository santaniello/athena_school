package exercise

import (
	"context"
	"fmt"
	"slices"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// CatalogService reads the challenge catalog: each challenge with the progress
// derived from its attempts. It never reveals a challenge's scenario,
// requirements or rubric. See
// specs/phases/phase-03-architecture-lab/07-challenge-catalog-ui.md.
type CatalogService struct {
	challenges domainexercise.ChallengeRepository
	records    domainexercise.AttemptRecordRepository
}

// NewCatalogService creates a CatalogService backed by the given ports.
func NewCatalogService(challenges domainexercise.ChallengeRepository, records domainexercise.AttemptRecordRepository) *CatalogService {
	return &CatalogService{challenges: challenges, records: records}
}

// CatalogItem is one row of the catalog: what does not give the exam away,
// plus the challenge's progress.
type CatalogItem struct {
	ChallengeID      string
	Kind             domainexercise.ChallengeKind
	Title            string
	Topic            string
	Difficulty       domainexercise.Difficulty
	TimeLimitMinutes int // 0 for an untimed practice
	Status           domainexercise.ProgressStatus
	Attempts         int
	BestDiagramScore int
}

// Catalog is the listed items plus the counts shown above them. The counts
// follow the status, difficulty and topic filters but not the text search.
type Catalog struct {
	Total      int
	Completed  int
	InProgress int
	Items      []CatalogItem
}

// CatalogFilter narrows the catalog. Every non-empty field must match (AND).
type CatalogFilter struct {
	Query      string // over title, scenario and topic
	Status     domainexercise.ProgressStatus
	Difficulty domainexercise.Difficulty
	Topic      string
}

// catalogEntry is a challenge in scope with its derived progress.
type catalogEntry struct {
	item    CatalogItem
	running bool
}

// ListCatalog lists the challenges matching the filter. Status is derived, so
// it is filtered here, after the challenges are loaded; the result is small
// enough to be held in memory. A challenge with an attempt running comes
// first; the others keep the repository order, most recently updated first.
func (s *CatalogService) ListCatalog(ctx context.Context, filter CatalogFilter) (Catalog, error) {
	scope, err := s.listAll(ctx, domainexercise.ChallengeFilter{Topic: filter.Topic, Difficulty: filter.Difficulty})
	if err != nil {
		return Catalog{}, err
	}
	matching, err := s.matchingQuery(ctx, filter)
	if err != nil {
		return Catalog{}, err
	}
	entries, err := s.deriveEntries(ctx, scope)
	if err != nil {
		return Catalog{}, err
	}

	catalog := Catalog{}
	var running, others []CatalogItem
	for _, entry := range entries {
		if filter.Status != "" && entry.item.Status != filter.Status {
			continue
		}
		catalog.count(entry.item.Status)
		if matching != nil && !matching[entry.item.ChallengeID] {
			continue
		}
		if entry.running {
			running = append(running, entry.item)
		} else {
			others = append(others, entry.item)
		}
	}
	catalog.Items = append(append([]CatalogItem{}, running...), others...)
	return catalog, nil
}

func (c *Catalog) count(status domainexercise.ProgressStatus) {
	c.Total++
	switch status {
	case domainexercise.ProgressCompleted:
		c.Completed++
	case domainexercise.ProgressInProgress:
		c.InProgress++
	}
}

// ListTopics returns the distinct topics of all challenges, for the topic filter.
func (s *CatalogService) ListTopics(ctx context.Context) ([]string, error) {
	topics, err := s.challenges.ListTopics(ctx)
	if err != nil {
		return nil, fmt.Errorf("exercise: listing topics: %w", err)
	}
	return topics, nil
}

// matchingQuery returns the ids of the challenges the text search matches
// within the same difficulty and topic, or nil when there is no search.
func (s *CatalogService) matchingQuery(ctx context.Context, filter CatalogFilter) (map[string]bool, error) {
	if filter.Query == "" {
		return nil, nil
	}
	found, err := s.listAll(ctx, domainexercise.ChallengeFilter{Query: filter.Query, Topic: filter.Topic, Difficulty: filter.Difficulty})
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(found))
	for _, challenge := range found {
		ids[challenge.ID] = true
	}
	return ids, nil
}

// listAll reads every page of the challenges matching the filter.
func (s *CatalogService) listAll(ctx context.Context, filter domainexercise.ChallengeFilter) ([]domainexercise.Challenge, error) {
	var all []domainexercise.Challenge
	for {
		page, err := s.challenges.List(ctx, filter, domainexercise.Page{Limit: domainexercise.MaxPageLimit, Offset: len(all)})
		if err != nil {
			return nil, fmt.Errorf("exercise: listing challenges: %w", err)
		}
		all = append(all, page.Items...)
		if len(page.Items) == 0 || len(all) >= page.Total {
			return all, nil
		}
	}
}

// deriveEntries loads the attempts of all the challenges in one go and derives
// each challenge's progress.
func (s *CatalogService) deriveEntries(ctx context.Context, challenges []domainexercise.Challenge) ([]catalogEntry, error) {
	if len(challenges) == 0 {
		return nil, nil
	}
	ids := make([]string, len(challenges))
	for i, challenge := range challenges {
		ids[i] = challenge.ID
	}
	records, err := s.records.ListByChallenges(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("exercise: reading attempts of challenges: %w", err)
	}
	entries := make([]catalogEntry, len(challenges))
	for i, challenge := range challenges {
		entries[i], err = deriveEntry(challenge, records[challenge.ID])
		if err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func deriveEntry(challenge domainexercise.Challenge, records []domainexercise.AttemptRecord) (catalogEntry, error) {
	summaries := make([]domainexercise.AttemptSummary, len(records))
	for i, record := range records {
		summary, err := summarizeRecord(record)
		if err != nil {
			return catalogEntry{}, err
		}
		summaries[i] = summary
	}
	progress := domainexercise.DeriveProgress(summaries)
	return catalogEntry{
		running: slices.ContainsFunc(summaries, func(summary domainexercise.AttemptSummary) bool { return summary.Running }),
		item: CatalogItem{
			ChallengeID: challenge.ID, Kind: challenge.Kind, Title: challenge.Title, Topic: challenge.Topic,
			Difficulty: challenge.Difficulty, TimeLimitMinutes: challenge.TimeLimitMinutes,
			Status: progress.Status, Attempts: progress.Attempts, BestDiagramScore: progress.BestDiagramScore,
		},
	}, nil
}
