package knowledge

import (
	"context"
	"errors"
	"math"
)

// Source-mode values accepted by study.Service.SendMessage. The mode is
// transient — passed per call, never stored or inferred from history. See
// specs/phases/phase-02-knowledge-engine/05-rag-integration.md.
const (
	SourceModeNotes       = "notes"
	SourceModeStrictNotes = "strict-notes"
	SourceModeWeb         = "web"
)

// ErrInvalidSourceMode is returned when a caller passes a source mode other
// than SourceModeNotes, SourceModeStrictNotes, or SourceModeWeb, before the
// user message is persisted and before any embedding, retrieval, or chat
// call.
var ErrInvalidSourceMode = errors.New("unknown source mode")

// NoLocalKnowledgeMessage is the fixed assistant response persisted and
// streamed back when strict-notes retrieval succeeds but no chunk survives
// filtering — the only local mode with no surviving chunks that makes no
// chat/completion call.
const NoLocalKnowledgeMessage = "No local knowledge found for this question."

// Defaults calibrated for text-embedding-3-small. Surfacing them in Settings
// is outside this phase.
const (
	// DefaultTopK is the number of chunks requested from VectorStore.Search.
	DefaultTopK = 8
	// DefaultMinSimilarity discards chunks scoring below it.
	DefaultMinSimilarity = 0.35
	// DefaultSufficiency is the score a surviving chunk must meet or exceed
	// for a RetrievalResult to be considered Sufficient.
	DefaultSufficiency = 0.55
)

// ErrInvalidThresholds is returned by NewRetrievalThresholds when either
// value is not finite, falls outside the cosine range [-1, 1], or minScore
// exceeds sufficiencyScore.
var ErrInvalidThresholds = errors.New("invalid retrieval thresholds")

// RetrievalThresholds holds the two score cutoffs a Retriever applies to
// search results. Constructed once via NewRetrievalThresholds so every
// holder already carries a validated value — there is no exported
// constructor bypass.
type RetrievalThresholds struct {
	MinSimilarity float64
	Sufficiency   float64
}

// NewRetrievalThresholds validates minScore and sufficiencyScore and
// returns them as a RetrievalThresholds, or ErrInvalidThresholds if either
// is non-finite, outside [-1, 1], or minScore exceeds sufficiencyScore.
// Equality between the two is allowed.
func NewRetrievalThresholds(minScore, sufficiencyScore float64) (RetrievalThresholds, error) {
	if !validCosineScore(minScore) || !validCosineScore(sufficiencyScore) {
		return RetrievalThresholds{}, ErrInvalidThresholds
	}
	if minScore > sufficiencyScore {
		return RetrievalThresholds{}, ErrInvalidThresholds
	}
	return RetrievalThresholds{MinSimilarity: minScore, Sufficiency: sufficiencyScore}, nil
}

func validCosineScore(score float64) bool {
	return !math.IsNaN(score) && !math.IsInf(score, 0) && score >= -1 && score <= 1
}

// Source is the desktop-facing description of one surviving chunk: enough
// for the UI to attribute an answer to its local material, without
// exposing internal IDs or the full excerpt's storage details.
type Source struct {
	ChunkID    string
	ItemID     string
	SourceType string // user_note | imported_doc | athena
	FilePath   string
	Heading    string
	Concept    string
	Score      float32
	Excerpt    string
}

// RetrievalResult is what a Retriever returns for one query: the
// post-cap surviving chunks, whether they are Sufficient, the already
// capped and rendered JSON data block, and the matching Source list —
// Chunks, Context, and Sources always describe exactly the same set, in
// the same score-descending/ID-ascending order.
type RetrievalResult struct {
	Chunks     []ScoredChunk
	Sufficient bool
	Context    string // deterministic JSON data block, already capped
	Sources    []Source
}

// Retriever performs one local-knowledge-base retrieval for a study
// session's query. study.Service owns source-mode policy and does not call
// Retriever at all for SourceModeWeb, so Retriever needs no mode parameter.
type Retriever interface {
	Retrieve(ctx context.Context, sessionID, query string) (RetrievalResult, error)
}
