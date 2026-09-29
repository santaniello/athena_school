package ingest

import (
	"context"
	"errors"
	"fmt"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// ErrSourceTextUnavailable is returned by GetSourceDocument when itemID's
// ownership checks out but no document text is stored for it — a chunk
// imported before spec 2.18, or one whose text was otherwise lost. See
// specs/phases/phase-02-knowledge-engine/18-notebooklm-style-citations.md
// decision 8.
var ErrSourceTextUnavailable = errors.New("ingest: source text unavailable")

// GetSourceDocument returns itemID's full document, cut into ordered
// segments at its own chunks' boundaries, for the source viewer. itemID not
// existing, or belonging to another session, is reported the same way —
// ErrSourceNotFound — as RemoveSource's ownership check, before any read of
// the document text.
func (s *Service) GetSourceDocument(ctx context.Context, sessionID, itemID string) (domainknowledge.SourceDocument, error) {
	item, err := s.items.GetByID(ctx, itemID)
	if err != nil {
		if errors.Is(err, domainknowledge.ErrItemNotFound) {
			return domainknowledge.SourceDocument{}, ErrSourceNotFound
		}
		return domainknowledge.SourceDocument{}, fmt.Errorf("ingest: looking up source: %w", err)
	}
	if item.SessionID != sessionID {
		return domainknowledge.SourceDocument{}, ErrSourceNotFound
	}

	content, err := s.documents.Get(ctx, sessionID, itemID)
	if err != nil {
		if errors.Is(err, domainknowledge.ErrDocumentNotFound) {
			return domainknowledge.SourceDocument{}, ErrSourceTextUnavailable
		}
		return domainknowledge.SourceDocument{}, fmt.Errorf("ingest: getting source text: %w", err)
	}

	chunks, err := s.chunks.ListByItemID(ctx, itemID)
	if err != nil {
		return domainknowledge.SourceDocument{}, fmt.Errorf("ingest: listing source chunks: %w", err)
	}

	path := ""
	if len(chunks) > 0 {
		path = chunks[0].FilePath
	}

	return domainknowledge.SourceDocument{
		ItemID:   itemID,
		Title:    item.Concept,
		Path:     path,
		Segments: buildSegments(content, chunks),
	}, nil
}

// buildSegments cuts content into ordered segments at chunks' own
// boundaries: a chunk's own passage (ChunkID set) or the gap before the
// first chunk, between two chunks, or after the last one (ChunkID empty).
// Segments reassemble to content exactly. chunks is assumed already in
// document order (ChunkRepository.ListByItemID's contract).
//
// A chunk with no offset (imported before spec 2.18) or whose offsets
// don't fit content — before the current cursor, inverted, or past the end
// of content — is skipped defensively rather than corrupting the walk:
// ImportFile always writes a chunk's offsets and its document's text
// together, atomically, so this should never trigger in practice.
func buildSegments(content string, chunks []domainknowledge.Chunk) []domainknowledge.DocumentSegment {
	var segments []domainknowledge.DocumentSegment
	cursor := 0
	for _, chunk := range chunks {
		if chunk.StartOffset == nil || chunk.EndOffset == nil {
			continue
		}
		start, end := *chunk.StartOffset, *chunk.EndOffset
		if start < cursor || end < start || end > len(content) {
			continue
		}
		if start > cursor {
			segments = append(segments, domainknowledge.DocumentSegment{Text: content[cursor:start]})
		}
		segments = append(segments, domainknowledge.DocumentSegment{Text: content[start:end], ChunkID: chunk.ID})
		cursor = end
	}
	if cursor < len(content) {
		segments = append(segments, domainknowledge.DocumentSegment{Text: content[cursor:]})
	}
	return segments
}
