# Code Conventions

## Naming Conventions

- **Packages:** short, lowercase, single-word, matching the directory name — `jira`, `toggl`, `sync`, `webhook`, `config`, `di`, `logger`, `server`, `telemetry`.
- **Files:** lowercase, `snake_case` for multi-word, feature-scoped — `process_delete.go`, `tokenexpiry.go`, `dependency.go`. Test files mirror the source file with a `_test.go` suffix (`worklog.go` → `worklog_test.go`); a test-only support file uses a `_test.go` suffix on a descriptive name (`fake_client_test.go`).
- **Functions/Methods:** `PascalCase` exported, `camelCase` unexported — `ParseDescription`, `NormalizeEntry`, `verifySignature`, `classifyStatus`. Constructors are `New<Type>` (`NewClient`, `NewHandler`, `NewProcessor`).
- **Test functions:** `Test<Subject>_<Scenario>` — e.g. `TestFindWorklogByTogglID_Found`, `TestNormalizeEntry_DeleteShapeA_DescriptionPresentNoDuration`, `TestDo_SetsBasicAuthHeader`. Scenario names are specific enough to double as documentation of the case covered.
- **Variables:** `camelCase`; short receiver names matching the type's first letter(s) (`c *Client`, `p *Processor`, `h *Handler`, `d *Dependency`, `e Event`).
- **Constants:** `camelCase` for unexported package-level constants (`defaultClientTimeout`, `signatureHeader`, `jiraTimeLayout`); grouped `const ( ... )` blocks with a doc comment on the block when the constants share a purpose (e.g. HTTP server timeouts in `main.go`).
- **Outcome/status values:** exported string constants prefixed by category — `sync.OutcomeCreated`, `sync.OutcomeSkippedInvalid`, etc. — rather than bare string literals, so callers and tests reference the same symbol.
- **Routes:** REST-ish path segments, singular resource nesting — `POST /webhooks/toggl`.
- **Branches:** `feature/<TICKET-ID>_<slug>` — e.g. `feature/TOGGL-1_webhook`.

## Code Organization

- **Import ordering:** stdlib first, blank line, then third-party (`gin`, `otel/*`), blank line, then this module's own `internal/...` packages — consistently across every source file read.
- **File structure:** package doc comment (when the file is the package's primary file) → imports → constants → types → constructor → methods, in that order. One logical concern per file (`worklog.go` = CRUD, `adf.go` = ADF marshaling, `tokenexpiry.go` = expiry check, all within `internal/jira`).
- **Interfaces defined at the consumer, not the implementer** — `sync.JiraClient` lives in `internal/sync` (who needs it), not `internal/jira` (who implements it), specifically to keep the implementer package free of test-only concerns and let the consumer's tests fake it.

## Type Safety / Documentation

- Statically typed Go throughout; no code generation or reflection-heavy patterns beyond `validator/v10` struct tags and standard `encoding/json` tags.
- Pointer fields are used deliberately to distinguish "absent" from "zero value" in inbound wire types where that distinction matters (`toggl.TimeEntryPayload`'s `Description`, `Duration`, `Start`, `Stop`), documented inline with why.
- Every exported symbol in every package read carries a doc comment starting with the symbol's name, per Go convention (`// ParseDescription extracts...`, `// NewHandler constructs...`).

## Error Handling

- Errors are wrapped with `fmt.Errorf("<package>: <action>: %w", err)`, consistently namespaced by package — `"jira: encode request body: %w"`, `"config: invalid value for %s: ..."`, `"telemetry: trace exporter: %w"`, `"toggl: parse envelope: %w"`, `"di: %w"`.
- Multiple independent errors accumulated during a single pass (e.g. config field parsing) are collected into a slice and joined with `errors.Join` rather than returning on first failure — see `config.Load`.
- Sentinel/typed errors are used where the caller needs to branch on error kind: `jira.TransientError` / `jira.PermanentError` (both implement `Error()`/`Unwrap()`), constructed by `classifyStatus` from the HTTP status code, letting `sync.Processor` (and future callers) distinguish retryable from non-retryable JIRA failures without string matching.
- Handler-level errors (malformed body, bad signature, malformed JSON) are logged via `slog` and mapped directly to an HTTP status — never propagated as a Go `error` back up through gin.
- No panics for control flow anywhere in the reviewed code; `gin.Recovery()` is the only panic boundary, applied once in `main.go`.

## Comments / Documentation

- Package-level doc comments explain the package's role in one to three sentences, on the file most representative of the package's purpose (`adf.go` for `jira`, `process.go` for `sync`, `handler.go` for `webhook`).
- Exported-symbol doc comments explain *why*, not just *what*, when the choice is non-obvious — e.g. `webhook.Receive`'s comment on reading the raw body before JSON binding explains the HMAC-verification gotcha it's avoiding, with a pointer back to `design.md`.
- Inline comments are used sparingly, reserved for non-obvious invariants (e.g. `routes.Routes`'s comment on why routes are registered on the group, not the engine) — not restating what a line already says.
- Requirement IDs (`TJ-01`, `TJ-05`, etc.) are referenced directly in doc comments on the functions that implement them, tying code back to `.specs/features/to-jira/spec.md`'s traceability table.

## Documentation Pattern

- No OpenAPI/Swagger spec or generated API docs — the single HTTP route is documented via the requirement traceability table in `.specs/features/to-jira/spec.md` and doc comments on `webhook.Handler.Receive`.
- Feature-level documentation lives in `.specs/features/to-jira/` (spec, design, tasks, validation) via the `tlc-spec-driven` skill — out of scope for this doc set to modify, but a primary source for it.
- Deliberately deferred edge cases are tracked in `to-jira/docs/NOT_IMPLEMENT.md` rather than as inline `TODO`/`FIXME` comments, each with scenario, why it's not handled, actual impact, and what would close it.
