package knowledge

import "context"

// MessageSourceRepository persists and restores the Sources that backed one
// completed study session message. messageID/sessionID are plain, opaque
// strings — this package has no dependency on domain/study.Message, so the
// message+Sources join happens in the application layer instead (see
// specs/phases/phase-02-knowledge-engine/09-persistent-provenance.md).
// Implemented by internal/infrastructure/sqlite.
type MessageSourceRepository interface {
	// Save replaces every Source previously persisted for messageID with
	// sources, in the given order. An empty slice is a valid no-op write —
	// never an error — for a message that used no local knowledge.
	Save(ctx context.Context, messageID string, sources []Source) error
	// ListBySession returns every persisted Source for every message in
	// sessionID, keyed by message ID, each already in the order Save
	// received them. A message with no persisted sources is absent from
	// the map, never present with an empty slice.
	ListBySession(ctx context.Context, sessionID string) (map[string][]Source, error)
}
