package knowledge

import "context"

// QueryEnricher produces a semantically richer version of a retrieval
// query — a text expected to compare more favorably against long-form
// chunk content than a short natural-language question does on its own, by
// a HyDE-style implementation or another technique entirely. The returned
// text is meant to be embedded and searched with, as an additional query
// alongside the original one, never as a replacement for it — it is never
// persisted or shown to the user. See
// specs/phases/phase-02-knowledge-engine/05-01-hyde-query-enrichment.md.
//
// QueryEnricher is deliberately independent of Retriever and carries no
// notion of source mode: callers decide when enrichment is worth its cost
// (today, only study.Service's strict-notes turns), the same way they
// already decide what to do with a RetrievalResult.
type QueryEnricher interface {
	Enrich(ctx context.Context, sessionID, topic, content string) (string, error)
}
