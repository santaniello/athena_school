// Package reset holds the port for permanently clearing locally-stored
// study and knowledge data. See
// specs/phases/phase-01-desktop-mvp/13-reset-local-data.md.
package reset

import "context"

// Resetter permanently deletes every study session, message, non-default
// folder, and knowledge-domain row (items, chunks, evidence, reconciliation
// proposals, ingested files) from local storage. It never touches the
// default folder, the OpenRouter config, or the onboarding profile.
// Implemented by internal/infrastructure/sqlite.
type Resetter interface {
	Reset(ctx context.Context) error
}
