package knowledge

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Evidence origin types: OriginSessionMessage snapshots a quote from a Study
// Session Message; OriginKnowledgeChunk is reserved for a future flow that
// promotes a Knowledge Chunk into a richer, LLM-structured Item.
const (
	OriginSessionMessage = "session_message"
	OriginKnowledgeChunk = "knowledge_chunk"
)

// Evidence persistence invariant errors, returned by Evidence.Validate.
var (
	ErrEvidenceIDRequired          = errors.New("evidence id is required")
	ErrEvidenceOriginTypeInvalid   = errors.New("evidence origin type is invalid")
	ErrEvidenceOriginIDRequired    = errors.New("evidence origin id is required")
	ErrEvidenceSourceLabelRequired = errors.New("evidence source label is required")
	ErrEvidenceExcerptRequired     = errors.New("evidence excerpt is required")
	ErrEvidenceCreatedAtRequired   = errors.New("evidence created at is required")
)

// Evidence is an immutable snapshot of text that originated a Knowledge Item.
type Evidence struct {
	ID          string
	OriginType  string
	OriginID    string
	SourceLabel string
	Excerpt     string
	CreatedAt   time.Time
}

// ItemEvidence links a Knowledge Item to one immutable Evidence snapshot.
type ItemEvidence struct {
	ItemID     string
	EvidenceID string
}

// EvidenceRef is an unpersisted literal quote proposed during extraction.
type EvidenceRef struct {
	MessageID string
	Quote     string
}

// IsSupportedBy reports whether content contains ref's Quote verbatim — the
// invariant every Evidence reference must satisfy both when a candidate is
// first extracted and again, against the Message's then-current content,
// when it is later saved.
func (ref EvidenceRef) IsSupportedBy(content string) bool {
	return strings.Contains(content, ref.Quote)
}

// EvidenceRepository persists immutable Evidence snapshots and their Item links.
type EvidenceRepository interface {
	GetOrCreate(ctx context.Context, evidence Evidence) (Evidence, error)
	LinkToItem(ctx context.Context, link ItemEvidence) error
	ListByItem(ctx context.Context, itemID string) ([]Evidence, error)
	DeleteUnreferenced(ctx context.Context) error
}

// Validate checks the persistence invariants of an Evidence snapshot.
func (e Evidence) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return ErrEvidenceIDRequired
	}
	if e.OriginType != OriginSessionMessage && e.OriginType != OriginKnowledgeChunk {
		return ErrEvidenceOriginTypeInvalid
	}
	if strings.TrimSpace(e.OriginID) == "" {
		return ErrEvidenceOriginIDRequired
	}
	if strings.TrimSpace(e.SourceLabel) == "" {
		return ErrEvidenceSourceLabelRequired
	}
	if strings.TrimSpace(e.Excerpt) == "" {
		return ErrEvidenceExcerptRequired
	}
	if e.CreatedAt.IsZero() {
		return ErrEvidenceCreatedAtRequired
	}
	return nil
}
