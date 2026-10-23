# Phase 2.31 — Per-feature LLM model settings

## Goal

Let the user choose which model each AI feature uses, instead of the models being hardcoded in
`domain/llm/router.go`. The current tier defaults stay as the fallback, so a user who never opens
the setting sees no change. The embedding model is also configurable, behind a prominent warning
that changing it requires re-importing every note.

## Survey: where models are used today

Every call goes through the single OpenRouter client
(`infrastructure/openrouter/client.go`), which resolves the model with `domainllm.ModelFor(task)`.

| Feature | Task | Default (tier → model) | Caller |
|---|---|---|---|
| Study chat (streaming, opening turn) | `TaskStudy` | medium → `anthropic/claude-sonnet-4.5` | `application/study/stream.go` |
| Conversation summary | `TaskConversationSummary` | cheap → `openai/gpt-4o-mini` | `application/study/summarizer.go` |
| Query enrichment (HyDE) | `TaskQueryEnrichment` | cheap → `openai/gpt-4o-mini` | `application/knowledge/query_enrichment.go` |
| Note formatting on import | `TaskNoteFormatting` | medium → `anthropic/claude-sonnet-4.5` | `application/ingest/preview_import.go` |
| Embeddings (indexing and query) | n/a | `openai/text-embedding-3-small` (constant) | `application/ingest/import_file.go`, `application/knowledge/retrieval.go`, `main.go` |

`TaskOnboarding`, `TaskChallengeFeedback`, `TaskInterviewEvaluation` and `TaskComplexReasoning`
are declared but have no caller yet. They are not user-configurable in this increment; they keep
resolving through their tier.

## Decisions

1. **Granularity is per feature.** Four chat settings (study chat, conversation summary, query
   enrichment, note formatting) plus the embedding model. Tiers remain the defaults, not the
   user-facing concept.
2. **Empty means default.** A setting left empty resolves to the task's tier model
   (`tierModels`) or to `EmbeddingModel`. Existing installs and a missing `models` section in
   `config.yaml` therefore behave exactly as today.
3. **Resolution is a domain port.** `domain/llm` gains `ModelResolver` (`ModelFor(Task) string`,
   `EmbeddingModel() string`). `openrouter.Client` and the application services receive it instead
   of calling the package-level `ModelFor` / reading the `EmbeddingModel` constant. The domain
   keeps the defaults and does not import `config` (ADR-001); the resolver implementation is
   built in `main.go` from the persisted config.
4. **Settings are saved immediately and take effect on the next call.** The resolver reads the
   current values on each call (guarded by a mutex), so no restart is needed. In-flight streams
   keep the model they started with.
5. **Persistence in `~/.athena/config.yaml`.** A new `models` section next to `openrouter_key`;
   `domain/config.Config` gains the corresponding fields. No SQLite table: these are app-level
   preferences, like the API key, not session data.
6. **Model IDs are validated against the catalog.** Saving a model ID checks it against the
   existing `ModelCatalog` (`ListModels`, already used by `modelcatalog.Service`). An unknown ID is
   rejected with a clear error. If the catalog cannot be fetched, saving is refused rather than
   trusting an unverified ID. The UI selector is fed by the same catalog.
7. **The free fallback is unchanged.** `FreeFallbackModel` still takes over when the account has
   no credits, whatever the user chose.
8. **The summary window follows the chosen model.** `SummaryModelWindowTokens` (128k) is assumed
   by the summarizer and the context builder. It becomes the context length of the configured
   summary model, taken from the catalog (`ModelContextResolver`); 128k stays the fallback when
   the length is unknown.
9. **Changing the embedding model invalidates the index, and the user must re-import.** Chunks and
   `ingested_files` already store their `EmbeddingModel`. `IndexLoader.ListCurrent` only loads
   chunks of the current model, and import skips a file only when its stored model equals the
   current one. So after a change, old chunks stop being used for retrieval and a re-import
   re-embeds every note with the new model. Vectors from different models are never mixed
   (dimensions can differ, see `vectorstore/store.go`).
10. **Old chunks are kept, not deleted.** Consistent with the decoupled import lifecycle (notes
    are never removed automatically), switching the embedding model back restores the previous
    index without re-importing. Re-importing with the new model must replace the file's chunks
    for that model; whether stale chunks of other models are cleaned up on re-import is verified
    during implementation and, if they would accumulate, handled by a follow-up.
11. **Big warning before an embedding change.** Changing the embedding model opens a blocking
    confirmation dialog stating plainly: retrieval will stop returning existing notes until every
    note is re-imported, re-importing consumes API credits, and until then answers will not be
    grounded in the user's notes. The change is applied only after an explicit confirm; cancel
    leaves the previous value. The chat-model settings need no such dialog.
12. **`IndexLoader` follows the current embedding model.** It is created in `main.go` with the
    embedding model; it must read the resolver's current value (or be rebuilt/reloaded) so that
    retrieval after a change loads the right chunks.
13. **Usage tracking is unaffected.** `usage` already records the model per call, and the
    session's `Context.Model` already tolerates model changes.

## Config shape

```yaml
openrouter_key: sk-or-...
models:
  study: anthropic/claude-sonnet-4.5
  conversation_summary: openai/gpt-4o-mini
  query_enrichment: openai/gpt-4o-mini
  note_formatting: anthropic/claude-sonnet-4.5
  embedding: openai/text-embedding-3-small
```

Every key is optional; absent or empty means the default.

## New backend surface

- `domain/llm`: `ModelResolver` port; `DefaultModelResolver` (tier defaults) kept for tests and as
  the fallback. `ModelFor` becomes a method of the resolver.
- `domain/config`: `Config.Models` (per-task IDs) and `Config.EmbeddingModel`.
- `infrastructure/configfile`: read and write the `models` section, preserving `openrouter_key`.
- `application/settings` (or the existing config/onboarding service, per spec 08): use cases
  `GetModelSettings`, `SaveModelSettings` (validates against the catalog, returns whether the
  embedding model changed), with the resolver updated after a successful save.
- `infrastructure/openrouter`: client resolves models through `ModelResolver`.
- `application/ingest`, `application/knowledge`, `main.go`: use `resolver.EmbeddingModel()` instead
  of the constant.
- `interfaces/desktop`: thin Wails bindings for get/save and for listing selectable models.
- Mocks via `make mock`.

## UI

- In the settings screen (spec 08), a "Models" section with one selector per feature, each showing
  the current value, its default and a "Reset to default" action. Options come from the catalog,
  with search, since the catalog is large.
- The embedding selector sits in a separate block marked as advanced, with the confirmation dialog
  from decision 11. UI text is English; accessible names are unique per selector.

## Tasks

1. Domain: `ModelResolver` + default implementation, with tests for tier fallback, empty means
   default and unknown task defaulting to medium.
2. Config: new fields and `configfile` round trip, including a legacy file with only
   `openrouter_key`.
3. `openrouter.Client` and the application callers switch to the resolver (existing tests keep
   passing with the default resolver; new tests assert the configured model is sent in the
   request and recorded in `usage`).
4. Settings use cases: catalog validation, unknown ID rejected, catalog failure refuses the save,
   `EmbeddingModelChanged` reported only when the value actually changed, resolver updated on save.
5. Summary window from the configured model's context length with the 128k fallback.
6. Embedding change: `IndexLoader` follows the current model; integration test that after a change
   old chunks are not retrieved and a re-import re-embeds with the new model; switching back
   restores retrieval without re-import.
7. Wails bindings and the settings UI, including the blocking confirmation dialog.
8. Docs: `CHANGELOG.md`.

## Acceptance Criteria

- With no `models` section, every feature uses exactly the models it uses today.
- Choosing a model for a feature makes the next call of that feature use it (visible in `usage`);
  in-flight streams are not affected.
- An unknown model ID, or any save while the catalog is unreachable, is rejected and persists
  nothing.
- The free fallback still triggers on insufficient credits regardless of the choice.
- Changing the embedding model requires confirming a dialog that lists the consequences; cancel
  keeps the old value.
- After an embedding change, retrieval no longer returns chunks of the previous model, a re-import
  indexes notes with the new model, and switching back restores the previous index.
- The conversation summary target scales with the configured summary model's context length.
- `domain` and `application` stay free of infrastructure imports; mutation testing meets the 80%
  thresholds.

## Out of Scope

- Per-session or per-message model choice in the chat.
- User-configurable models for the reserved tasks (onboarding, challenge, interview, complex
  reasoning) until they have callers.
- An "outdated notes" indicator or automatic bulk re-import after an embedding change.
- Providers other than OpenRouter.
- Cleaning up chunks of unused embedding models (revisit if they accumulate).
