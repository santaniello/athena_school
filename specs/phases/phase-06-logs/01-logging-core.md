# Phase 6.1 — Logging Core

## Goal

Build the centralized, file-only logging infrastructure as a self-contained package,
`internal/infrastructure/logging`. Nothing else in the application changes in this increment: no
module uses it yet, so the app behaves exactly as before. Config wiring is 6.2 and module migration
is 6.3.

## Decisions

1. **Standard library only** (`log/slog`, `os`, `path/filepath`, `time`, `io`, `sync`). No external
   dependency, no database, no GUI, no per-module files.
2. **One file per execution**, named `athena-YYYY-MM-DD_HH-mm-ss.log` (start time), inside
   `<athena home>/logs/`. The package receives the logs directory from the caller and never reads
   `os.UserHomeDir()` itself (see `athenahome`). On a name collision a numeric suffix is added
   (`_002`, `_003`…) so no previous file is ever overwritten.
3. **Permissions:** directory `0700`, files `0600` (AGENTS.md security rules). The directory is
   created when missing.
4. **Format: JSON Lines** through `slog.NewJSONHandler`. Every event carries `time` (with time zone),
   `level`, `module` and `msg`. Other attributes are optional (`error`, `duration_ms`, `session_id`,
   `challenge_id`, `attempt_id`, `provider`, `model`, `status_code`, `operation`, `source`).
5. **Levels:** DEBUG, INFO, WARN, ERROR. Default INFO; below the configured level nothing is
   written.
6. **Module identification:** the package returns a root logger; a module logger is
   `root.With("module", "<name>")`. A helper may build it, but there is no registry of modules.
7. **Writing:** unbuffered, one `Write` per event on the `os.File` under a mutex. Events already
   emitted survive a crash; no per-event `fsync`. Safe for concurrent goroutines.
8. **Size-based segmentation:** the writer counts the bytes of the active file. Before writing an
   event that would exceed `maxFileSize`, it closes the file and opens the next segment of the same
   execution (`athena-<start>_002.log`). An event is never split across files. The retention
   cleanup runs on every rotation.
9. **Retention** (runs once at startup, before the new file is created, and on every rotation):
   - Only files matching `^athena-\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}(_\d{3})?\.log$` belong to
     Athena; anything else in the directory is ignored and never deleted.
   - Delete historical files older than `retentionDays`. Age comes from the timestamp in the name,
     falling back to the modification time if the name cannot be parsed.
   - If the total size of the directory exceeds `maxTotalSize`, delete the oldest historical files
     until it fits. The active file counts toward the total but is never deleted; if it alone
     exceeds the limit, all historical files go and the run continues.
10. **Failure handling:** a failure creating or writing the file switches the writer to `stderr`
    (fallback). The failure is reported once on `stderr`, the writer does not try to recreate the
    file, and nothing is logged through the logger itself (no recursion). Cleanup failures are
    reported on `stderr` and never abort startup. A logging failure never stops the application.
11. **Defensive redaction** at the handler level, as a safety net on top of author discipline:
    - Values of attributes whose key is sensitive (`api_key`, `authorization`, `token`, `password`,
      `secret`, `credential`; case-insensitive) become `[REDACTED]`.
    - Raw content attributes (`prompt`, `response`, `content`, `note`) are redacted the same way.
      Modules log sizes or identifiers instead (`prompt_chars`, `tokens`).
    - String values, including `error`, are scanned for known credential patterns (OpenRouter
      `sk-or-…` keys, `Bearer <token>`) and the match is masked.
12. **Lifecycle:** `Close` flushes nothing (writes are unbuffered), closes the active file and is
    idempotent.
13. **Defaults** (a `Config` value type with these defaults; config loading is 6.2):

    | Setting | Default |
    |---|---|
    | Level | INFO |
    | Retention | 7 days |
    | Max total size | 100 MB |
    | Max file size | 20 MB |

## Implementation

```text
internal/infrastructure/logging/
├── config.go        Config value type and defaults
├── logger.go        New(logsDir, Config, now) → (*slog.Logger, io.Closer); handler wiring, Close
├── file_writer.go   file creation, collision suffix, size counting, rotation, stderr fallback
├── retention.go     athena file matching, age and size cleanup
├── redact.go        sensitive-key and credential-pattern redaction
└── logging_test.go
```

`now` (clock) and the logs directory are parameters so tests use `t.TempDir()` and a fixed time.
No abstractions beyond what these files need.

## Acceptance Criteria

- A new file is created per start; previous files are preserved and none is overwritten, including
  when two starts share the same second (suffix).
- Lines are valid JSON, each with `time`, `level`, `module` and `msg`.
- DEBUG is not persisted at INFO; all four levels work when enabled.
- Files older than the retention period are removed; the total size limit is respected; files that
  do not match the Athena pattern are untouched; the active file is never deleted.
- Rotation creates `_002`, `_003`… segments, never splits an event, and each file stays within the
  size limit (an event larger than the limit goes alone to a new file).
- 100 goroutines writing concurrently produce only complete, parseable lines.
- Known secrets (keys, bearer tokens, sensitive attribute keys) never appear in the file.
- With an unwritable directory the logger falls back to `stderr`, the application keeps running and
  there is no recursive logging.
- Directory `0700` and files `0600`.
- `Close` is idempotent.
- Same quality gate as the other phases (coverage ≥ 80%, lint, vulnerability check); no UI change.
