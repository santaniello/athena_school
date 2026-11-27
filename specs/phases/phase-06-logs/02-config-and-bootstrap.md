# Phase 6.2 — Config and Bootstrap

## Goal

Read the logging settings from `config.yaml`, start the logger in `main.go`, record the
application lifecycle events and close the log file on shutdown. After this increment the
application writes a log file on every run, but only the `application` module logs; other modules
still use `log.Printf` until 6.3.

Depends on [6.1](01-logging-core.md).

## Decisions

1. **Config lives in the existing mechanism:** an optional `logging:` section of
   `~/.athena/config.yaml`, read by `configfile` and modeled in `domain/config`, like the `models`
   section. Keys are snake_case:

   ```yaml
   logging:
     level: INFO
     retention_days: 7
     max_total_size_mb: 100
     max_file_size_mb: 20
     slow_operation_seconds: 30
     db_wait_warn_seconds: 5
     db_wait_timeout_seconds: 30
   ```

   The last three are used by the operation tracing and storage events of
   [6.3](03-module-migration.md) (decisions 8 to 12). They are read here so that 6.3 needs no new
   config work.

2. **Defaults for everything missing:** an absent section, an absent key or a zero value takes the
   default of 6.1 (for the three keys added for 6.3, the defaults of that spec: 30, 5 and 30 seconds). A negative number is invalid and takes the default. An invalid `level` takes
   INFO and a WARN is recorded once at startup. The settings load once at startup; no runtime
   change.
3. **The section is not written back** by `Save` when empty (`omitempty`), so existing config files
   are not changed by the settings screens, and saving the OpenRouter key or models never drops a
   user-edited `logging:` section.
4. **Startup order in `main.go`:**
   1. Resolve the Athena home and the `logs` directory.
   2. Load the config.
   3. Run retention, create the file and build the logger (6.1).
   4. Record `application`: "Athena starting" with the version, "Config loaded", "Athena ready"
      once the window is up.
   5. Failures before the logger exists go to `stderr`. The `log.Fatalf` paths of `main.go`
      record the error in the logger first (when it exists), then exit.
5. **Shutdown:** the logger records "Athena shutting down" and is closed in Wails `OnShutdown`
   (new hook) and in a `defer` in `main`; `Close` is idempotent (6.1), so both can run.
6. **Only `main.go` and the bootstrap touch `infrastructure/logging`.** The `main.go` and
   `context_override.go` `log.Printf` calls move to the `application` logger here; nothing
   else changes.
7. **If the logs directory cannot be used** the application starts normally with the `stderr`
   fallback (6.1 decision 10).

## Implementation

- `domain/config`: `Logging` struct (level string, retention days, max total MB, max file MB) in
  `Config`, with a method returning the effective values after defaulting.
- `configfile`: `logging` section read and written with `omitempty`.
- `main.go`: bootstrap as above; version comes from the existing version source (the one used by
  the auto-update check).
- Mapping from `domain/config` to `logging.Config` lives in `main.go` (the domain never imports
  infrastructure).

## Acceptance Criteria

- Without a `logging:` section every default applies.
- Partial sections keep user values and default the rest; invalid values fall back as described.
- Saving other settings preserves the user's `logging:` section.
- Running the app creates `~/.athena/logs/athena-<timestamp>.log` with the starting, config
  loaded and ready events, and, on exit, the shutting down event; the file is then closed.
- A second run creates a second file and keeps the first.
- With `level: DEBUG` debug events appear; with the default they do not.
- No `log.Printf`/`log.Fatalf` remains in `main.go` or `context_override.go` after the logger
  exists.
- Same quality gate as 6.1.
