package ingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	"github.com/santaniello/athena/internal/domain/web"
)

// urlReadPath is the fs-style name a link's candidate carries. Only its
// ".md" extension matters: ingestContent chunks by it, and a fetched page is
// always Markdown.
const urlReadPath = "link.md"

// PreviewURL fetches rawURL, converts it to Markdown and formats and chunks
// it exactly as PreviewImport does a .md file — without persisting anything.
// ConfirmURLImport is its only valid follow-up and must receive exactly the
// FormattedContent this returns. Like PreviewImport it only checks, never
// reserves, the index (spec 2.20 decision 9). There is no skip-unchanged
// check: a link has no modification time, so adding a URL again always
// previews and, once approved, replaces the earlier import.
//
// The preview's SourceURL is the canonical URL the source will be stored
// under and its DisplayPath is the page title, or the URL when the page has
// none. onProgress has PreviewImport's contract.
func (s *Service) PreviewURL(
	ctx context.Context, sessionID, rawURL string, onProgress func(Progress) error,
) (ImportPreview, error) {
	if strings.TrimSpace(sessionID) == "" {
		return ImportPreview{}, domainknowledge.ErrSessionRequired
	}
	canonical, err := web.CanonicalURL(rawURL)
	if err != nil {
		return ImportPreview{}, err
	}
	if err := s.index.CheckMutationAllowed(); err != nil {
		return ImportPreview{}, err
	}

	page, err := s.fetcher.Fetch(ctx, canonical)
	if err != nil {
		return ImportPreview{}, fmt.Errorf("ingest: fetching %s: %w", canonical, err)
	}

	preview, err := s.buildPreview(ctx, sessionID, linkDisplayPath(page.Title, canonical), page.Markdown, true, onProgress)
	if err != nil {
		return ImportPreview{}, err
	}
	preview.SourceURL = canonical
	return preview, nil
}

// ConfirmURLImport persists exactly approvedContent — the text a prior
// PreviewURL returned and the user approved — as the link's chunks and
// shadow Item. It never fetches or formats again, so what the user reviewed
// is what gets embedded. The URL is canonicalized here (never trusted from
// the caller as already canonical) and is the source's identity: confirming
// a URL that was imported before replaces that source instead of duplicating
// it. Like ConfirmImport it reserves the index for its whole call, and its
// progress and error contract is the same.
func (s *Service) ConfirmURLImport(
	ctx context.Context, sessionID, rawURL, title, approvedContent string, onProgress func(Progress) error,
) (Summary, error) {
	if strings.TrimSpace(sessionID) == "" {
		return Summary{}, domainknowledge.ErrSessionRequired
	}
	canonical, err := web.CanonicalURL(rawURL)
	if err != nil {
		return Summary{}, err
	}

	if err := s.index.BeginMutation(); err != nil {
		return Summary{}, err
	}
	defer s.index.EndMutation()

	if onProgress == nil {
		onProgress = func(Progress) error { return nil }
	}

	existing, err := s.ingestedFiles.ListBySession(ctx, sessionID)
	if err != nil {
		return Summary{}, fmt.Errorf("ingest: listing previously ingested files: %w", err)
	}
	prev, hasPrev := existing[canonical]

	candidate := importCandidate{ReadPath: urlReadPath, SourcePath: canonical}
	displayPath := linkDisplayPath(title, canonical)
	return s.persistApproved(
		ctx, sessionID, candidate, displayPath, time.Now().UnixNano(), prev, hasPrev, approvedContent, onProgress,
	)
}

// linkDisplayPath is the label a link source is listed under: its title, or
// its canonical URL when it has none.
func linkDisplayPath(title, canonicalURL string) string {
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		return trimmed
	}
	return canonicalURL
}
