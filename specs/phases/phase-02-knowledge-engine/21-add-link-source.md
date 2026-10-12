# Phase 2.21 — Add a Link as a Source

## Goal

A URL pasted into a Study chat message reaches the model as plain text: Athena never opens the
page, so the model can only guess at its content. This spec lets the user add a web page as a
session source from the Sources panel, the way NotebookLM does: **Add** gains a type selector —
**Markdown** (today's file flow, unchanged) or **Link** (new). The page is fetched, converted to
Markdown, passed through the same LLM formatting and Review dialog as an imported note (2.20 /
2.20.1), and persisted as an ordinary session source: chunks, embeddings, stored text for the
viewer, and `[n]` citations all work with no schema change. The user can then ask many questions
about the article without ever pasting the link again.

## Decisions

1. **Explicit intent, not chat detection.** The chat (`SendMessage`) does not change. A link is
   only fetched when the user picks **Add → Link**.
2. **A link is a source like a file.** It goes through `ingestContent`, so retrieval, strict-notes
   sufficiency, citations, the viewer and "Remove" need no special case. The source's identity
   (`SourcePath`) is the canonical URL (lowercased scheme/host, fragment removed), so adding the
   same URL again replaces it instead of duplicating it.
3. **Same Preview → Review → Confirm gate as Markdown.** `PreviewURL` fetches and formats without
   persisting; `ConfirmURLImport` persists exactly the approved text and never re-fetches or
   re-formats (2.20 decision 4). The formatting pipeline is shared: `PreviewImport`'s post-read
   half is extracted into `buildPreview`, used by both.
4. **No skip-unchanged for links.** There is no mtime to compare; re-adding a URL always previews
   and, if approved, replaces.
5. **The fetcher is a domain port** (`domain/web.Fetcher`) with an HTTP adapter in
   `infrastructure/webfetch`. The application layer never imports `net/http`.
6. **Fetch hardening (SSRF).** Only `http`/`https`; loopback, private, link-local and metadata
   addresses are rejected at dial time (covers redirects and DNS rebinding); bounded redirects,
   timeout, body size and extracted-text length; only `text/html` and `text/plain`.
7. **HTML → Markdown is deliberately simple** (`golang.org/x/net/html`): drops script, style,
   nav, footer and aside; keeps the title, headings, paragraphs, lists and code blocks. Residual
   noise is handled by the formatting prompt and the user's Review.
8. **The canonical URL is recomputed in the backend** on confirm; the frontend only echoes it.

## New/changed backend surface

- `domain/web`: `Fetcher.Fetch(ctx, rawURL) (Page, error)`, `Page{URL, Title, Markdown}`,
  `CanonicalURL`, errors `ErrInvalidURL`, `ErrBlockedHost`, `ErrUnsupportedContent`,
  `ErrTooLarge`, `ErrEmptyContent`.
- `infrastructure/webfetch`: the HTTP adapter.
- `application/ingest`: `PreviewURL`, `ConfirmURLImport`; `Service` gains a `web.Fetcher`.
- Desktop bindings: `PreviewLinkImport(sessionID, url)`, `ConfirmLinkImport(sessionID, url, title,
  approvedContent)`, reusing the `ingest:*` events.

## Frontend design

`AddSourceButton` becomes a menu: **Markdown** (existing flow) / **Link** (new `AddLinkDialog`
with a URL field). The link flow reuses `FormattingNoteDialog`, `ImportReviewDialog` and
`IngestProgressDialog`. The viewer shows an "open original" action for URL sources.

## Tasks

1. `domain/web` (types, errors, `CanonicalURL`).
2. `infrastructure/webfetch` (SSRF-safe client, HTML → Markdown).
3. Extract `buildPreview` from `PreviewImport` (behavior unchanged).
4. `PreviewURL` / `ConfirmURLImport`.
5. Desktop bindings and `main.go` wiring.
6. Frontend: menu, `AddLinkDialog`, wiring, viewer action.
7. Docs: `CHANGELOG.md`, README if needed.

## Acceptance Criteria

- Add → Link → paste a public article → Review → approve: the article is listed as a source and
  later questions retrieve it, in both `notes` and `strict-notes`, with working `[n]` citations.
- Re-adding the same URL replaces the source; nothing is duplicated.
- localhost, private IPs, non-HTTP schemes, oversized or non-text responses and empty pages fail
  with a clear message and persist nothing.
- Add → Markdown behaves exactly as before.

## Known risks

- Page text is untrusted: it is retrieved content and already goes through the untrusted-data
  framing of the retrieval block.
- SSRF is the main fetch risk; tested with redirects to private addresses.
- Pages that render only with JavaScript, or sit behind a login, come back empty.
- Questions like "summarize the whole article" depend on chunk retrieval, not the full text.

## Out of scope

Open web search / tool calling, detecting links in chat messages, JavaScript-rendered pages,
remote PDFs, login/paywall.
