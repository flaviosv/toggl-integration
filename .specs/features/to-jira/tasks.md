# to-jira Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user — do not proceed without it.**

---

**Design**: `.specs/features/to-jira/design.md`
**Status**: Draft

---

## Test Coverage Matrix

> Generated from codebase sampling of `~/Projects/Personal/dinherim` and `~/Projects/Personal/applyr` (no `CLAUDE.md`/`AGENTS.md`/`CONTRIBUTING.md` found in this repo — no guidelines to conform to yet; sibling-repo test conventions were already adopted into `design.md`'s Code Reuse Analysis, so they're treated as the floor here, not the strong-default fallback). Confirm before Execute.

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| --- | --- | --- | --- | --- |
| `internal/toggl` (parsing/validation) | unit | All branches; 1:1 to TJ-02/TJ-03 + every listed edge case (malformed formats, running entries, both hypothesized delete-payload shapes) | `internal/toggl/*_test.go` | `go test ./internal/toggl/...` |
| `internal/jira` (API client) | unit | All CRUD paths + ADF build/extract round-trip + transient vs. permanent error handling, via `httptest.Server` fakes — no live network calls | `internal/jira/*_test.go` | `go test ./internal/jira/...` |
| `internal/sync` (orchestration) | unit | Every `Result.Outcome` value reachable (`created`, `updated`, `skipped_invalid`, `skipped_running`, `deleted`, `noop`, `unsupported_delete`, `transient_error`) via a fake `jira.Client`; 1:1 to TJ-02…TJ-13 | `internal/sync/*_test.go` | `go test ./internal/sync/...` |
| `internal/webhook` (HTTP handler) | unit (`httptest`) | Signature verification (valid/invalid/missing) + dispatch + HTTP status mapping; happy + edge + error paths | `internal/webhook/*_test.go` | `go test ./internal/webhook/...` |
| `internal/shared/config` | unit | Required-field validation — missing/invalid env vars rejected at load | `internal/shared/config/*_test.go` | `go test ./internal/shared/config/...` |
| `internal/shared/{logger,telemetry,server,di}`, `internal/routes`, `main.go` | none | Thin wiring/bootstrap, no branching logic — build gate only | — | `go build ./...` |

## Gate Check Commands

> Generated from `dinherim`/`applyr` conventions (stdlib `testing`, `go build`/`go vet`/`go test` — no CI/lint tooling per the project's explicit no-CI decision, `go vet` included since it's a free stdlib check, not a CI dependency). Confirm before Execute.

| Gate Level | When to Use | Command |
| --- | --- | --- |
| Quick | After a task touching one package with unit tests | `go test ./internal/<pkg>/...` |
| Full | After a task wiring multiple packages together (webhook, di, main) | `go build ./... && go vet ./... && go test ./...` |
| Build | After each phase completes | `go build ./... && go vet ./... && go test ./...` |

---

## Execution Plan

Phases are ordered and run sequentially — each phase completes before the next begins, and tasks within a phase execute in order.

### Phase 1: Foundation

```
T1 → T2 → T3 → T4 → T5
```

### Phase 2: Toggl domain

```
T6 → T7 → T8
```

### Phase 3: JIRA client

```
T9 → T10 → T11 → T12 → T13 → T14 → T15
```

### Phase 4: Sync orchestration

```
T16 → T17
```

### Phase 5: Webhook + wiring

```
T18 → T19 → T20 → T21
```

---

## Task Breakdown

### T1: Initialize Go module and project skeleton

**What**: `go.mod` (module `github.com/flaviosv/toggl-integration/to-jira`, matching Go version to dinherim/applyr's `go 1.26.x`), `.gitignore` (`.env`, `bin/`), `.env.sample` listing every env var from design.md's Data Models section.
**Where**: `to-jira/go.mod`, `to-jira/.gitignore`, `to-jira/.env.sample`
**Depends on**: None
**Reuses**: dinherim/applyr's `.env.sample` shape
**Requirement**: N/A (scaffolding)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `go.mod` created with correct module path and Go version
- [ ] `.gitignore` excludes `.env` and build output
- [ ] `.env.sample` lists `TOGGL_WEBHOOK_SECRET`, `JIRA_BASE_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`, `JIRA_API_TOKEN_EXPIRES_AT`, `DRY_RUN`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `PORT`
- [ ] `go build ./...` succeeds on the empty module

**Tests**: none
**Gate**: build

---

### T2: Port `internal/shared/logger`

**What**: `Initialize(env string) *slog.Logger` (JSON handler in production, text in dev) + `FromContext(ctx)` pattern, copied from dinherim/applyr.
**Where**: `to-jira/internal/shared/logger/logger.go`
**Depends on**: T1
**Reuses**: `dinherim/internal/shared/logger/logger.go` (verbatim pattern)
**Requirement**: N/A (infra)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `Initialize`/`FromContext` match the reused pattern exactly
- [ ] `go build ./...` passes

**Tests**: none
**Gate**: build

---

### T3: Add `internal/shared/config`

**What**: Env-var loader (godotenv + `os.Getenv` + `validator/v10` struct tags) for all fields from T1's `.env.sample`, following dinherim/applyr's required-vs-optional-with-defaults policy.
**Where**: `to-jira/internal/shared/config/config.go` (+ `config_test.go`)
**Depends on**: T1
**Reuses**: `dinherim/internal/shared/config/config.go` pattern
**Requirement**: N/A (infra, gates all other requirements' credentials)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `TOGGL_WEBHOOK_SECRET`, `JIRA_BASE_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN` are required; missing any fails load with a clear error
- [ ] `JIRA_API_TOKEN_EXPIRES_AT`, `OTEL_EXPORTER_OTLP_ENDPOINT` are optional
- [ ] `DRY_RUN` defaults to `false`, `PORT` defaults sensibly (e.g. `8080`)
- [ ] Gate passes: `go test ./internal/shared/config/...`
- [ ] Test count: ≥4 tests (missing-required-field rejection ×N, defaults applied, full valid load)

**Tests**: unit
**Gate**: quick

---

### T4: Add `internal/shared/telemetry`

**What**: `Initialize(ctx, cfg) (shutdown func(context.Context) error, err error)` bootstrapping an OTel `TracerProvider` + `MeterProvider` — stdout exporter when `OTEL_EXPORTER_OTLP_ENDPOINT` is unset, OTLP exporter otherwise (AD-003). `Metrics` struct with the five counters from design.md.
**Where**: `to-jira/internal/shared/telemetry/telemetry.go`
**Depends on**: T3
**Reuses**: nothing existing (new for the stack, per AD-003)
**Requirement**: TJ-14

**Tools**:
- MCP: `context7` (verify current `go.opentelemetry.io/otel` SDK setup API — this is genuinely new to the stack, confidence threshold applies per current OTel Go SDK conventions)
- Skill: NONE

**Done when**:
- [ ] `Initialize` returns a working shutdown func, defaults to stdout exporter
- [ ] `Metrics` struct exposes `WorklogsCreated`, `WorklogsUpdated`, `WorklogsDeleted`, `ValidationErrors`, `JiraAPIErrors` as `metric.Int64Counter`
- [ ] `go build ./...` passes

**Tests**: none
**Gate**: build

---

### T5: Port `internal/shared/server`

**What**: gin engine construction + graceful shutdown (signal handling, timeout), copied from dinherim/applyr.
**Where**: `to-jira/internal/shared/server/server.go`
**Depends on**: T3
**Reuses**: `dinherim/internal/shared/server/server.go` (verbatim pattern)
**Requirement**: N/A (infra)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Server starts, binds `PORT` from config, shuts down gracefully on SIGTERM/SIGINT
- [ ] `go build ./...` passes

**Tests**: none
**Gate**: build

---

### T6: Define `internal/toggl` envelope/event types + `ParseEnvelope`

**What**: `WebhookEnvelope`, `EventMetadata`, `TimeEntryPayload` (pointer fields) structs from design.md's Data Models; `ParseEnvelope(body []byte) (WebhookEnvelope, error)`.
**Where**: `to-jira/internal/toggl/envelope.go` (+ `envelope_test.go`)
**Depends on**: T1
**Reuses**: nothing existing — new package
**Requirement**: TJ-01 (supports signature/dispatch), TJ-12 (payload shape)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `ParseEnvelope` correctly unmarshals a well-formed envelope
- [ ] `ParseEnvelope` returns an error on malformed JSON, not a panic
- [ ] Gate passes: `go test ./internal/toggl/...`
- [ ] Test count: ≥3 (valid envelope, malformed JSON, missing optional fields)

**Tests**: unit
**Gate**: quick

---

### T7: Implement `ParseDescription`

**What**: `ParseDescription(desc string) (issueKey, text string, ok bool)` against `^\[([A-Z][A-Z0-9]{1,9})-(\d+)\]\s*(.*)$` (TJ-02).
**Where**: `to-jira/internal/toggl/parse.go` (+ `parse_test.go`)
**Depends on**: T6
**Reuses**: nothing existing
**Requirement**: TJ-02

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Valid `[ABC-123] text` parses to `issueKey="ABC-123"`, `text="text"`, `ok=true`
- [ ] Missing brackets, lowercase slug, no number, irregular whitespace/punctuation all return `ok=false` (spec Edge Cases)
- [ ] Gate passes: `go test ./internal/toggl/...`
- [ ] Test count: ≥8 covering valid + every listed malformed-format edge case

**Tests**: unit
**Gate**: quick

---

### T8: Implement `NormalizeEntry`

**What**: `NormalizeEntry(p TimeEntryPayload) (Event, ok bool)` — `ok=false` when duration is negative or `Stop` is nil (TJ-03); builds both create/update entries and (defensively) delete-derived entries from partial payloads per the delete-payload hypotheses in design.md's Risks.
**Where**: `to-jira/internal/toggl/normalize.go` (+ `normalize_test.go`)
**Depends on**: T6
**Reuses**: nothing existing
**Requirement**: TJ-03, TJ-12

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Complete entry (positive duration, `Stop` set) normalizes with `ok=true`
- [ ] Running entry (negative duration or nil `Stop`) returns `ok=false`
- [ ] Payload with `Description` present but no duration (hypothesized delete shape A) still yields a usable `Event.Description` for issue derivation
- [ ] Payload with `Description` nil (hypothesized delete shape B) returns `ok=false` with a distinguishable reason the caller can map to `unsupported_delete`
- [ ] Gate passes: `go test ./internal/toggl/...`
- [ ] Test count: ≥6

**Tests**: unit
**Gate**: quick

---

### T9: Define `internal/jira` data types + ADF comment build/extract

**What**: `WorklogInput`, `Worklog`, `ADFDocument`/`ADFNode` structs; `BuildComment(togglID, text string) ADFDocument`; `ExtractTogglID(doc ADFDocument) (string, bool)` (first paragraph, first text run, prefix match).
**Where**: `to-jira/internal/jira/adf.go` (+ `adf_test.go`)
**Depends on**: T1
**Reuses**: nothing existing — first outbound API client in the stack
**Requirement**: supports TJ-05, TJ-06, TJ-10

**Tools**:
- MCP: `context7` (confirm current JIRA v3 ADF minimal-document schema)
- Skill: NONE

**Done when**:
- [ ] `BuildComment` produces a valid single-paragraph/single-text-run ADF doc matching `[TogglID:<id>] <text>`
- [ ] `ExtractTogglID` round-trips a `BuildComment` output correctly
- [ ] `ExtractTogglID` returns `ok=false` on a comment without the marker, and on a structurally different (manually-edited) ADF doc, per design.md's documented simplification
- [ ] Gate passes: `go test ./internal/jira/...`
- [ ] Test count: ≥5

**Tests**: unit
**Gate**: quick

---

### T10: Implement `jira.Client` construction + auth request helper

**What**: `type Client struct{...}`, `NewClient(baseURL, email, apiToken string, hc *http.Client) *Client`, an internal `do(ctx, method, path string, body any) (*http.Response, error)` applying HTTP Basic Auth.
**Where**: `to-jira/internal/jira/client.go` (+ `client_test.go`)
**Depends on**: T9
**Reuses**: nothing existing
**Requirement**: supports TJ-05, TJ-06, TJ-10

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `NewClient` stores config correctly
- [ ] `do` sets `Authorization: Basic ...` correctly for a known email/token pair (verified against a fake `httptest.Server`)
- [ ] `do` returns the raw response/error without swallowing non-2xx (callers decide transient vs. permanent)
- [ ] Gate passes: `go test ./internal/jira/...`
- [ ] Test count: ≥2

**Tests**: unit
**Gate**: quick

---

### T11: Implement `FindWorklogByTogglID`

**What**: `(c *Client) FindWorklogByTogglID(ctx, issueKey, togglID string) (*Worklog, error)` — `GET .../worklog`, list, filter via `ExtractTogglID`.
**Where**: `to-jira/internal/jira/worklog.go` (+ `worklog_test.go`)
**Depends on**: T10
**Reuses**: `ExtractTogglID` from T9
**Requirement**: TJ-04

**Tools**:
- MCP: `context7` (confirm JIRA v3 worklog list response shape/pagination)
- Skill: NONE

**Done when**:
- [ ] Returns the matching worklog when present, `nil` when absent, against an `httptest.Server` fake JIRA
- [ ] Handles a paginated worklog list (if JIRA's API paginates past a threshold — confirm via research, document assumption if uncertain)
- [ ] Gate passes: `go test ./internal/jira/...`
- [ ] Test count: ≥4

**Tests**: unit
**Gate**: quick

---

### T12: Implement `CreateWorklog`

**What**: `(c *Client) CreateWorklog(ctx, issueKey string, in WorklogInput) (*Worklog, error)` — `POST .../worklog`.
**Where**: `to-jira/internal/jira/worklog.go` (append)
**Depends on**: T11
**Reuses**: request helper from T10
**Requirement**: TJ-05

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Sends correct `timeSpentSeconds`, `started`, `comment` (ADF) body against a fake server
- [ ] Returns a distinguishable error on transient (5xx/network) vs. permanent (404/400) failure
- [ ] Gate passes: `go test ./internal/jira/...`
- [ ] Test count: ≥4

**Tests**: unit
**Gate**: quick

---

### T13: Implement `UpdateWorklog`

**What**: `(c *Client) UpdateWorklog(ctx, issueKey, worklogID string, in WorklogInput) error` — `PUT .../worklog/{id}`.
**Where**: `to-jira/internal/jira/worklog.go` (append)
**Depends on**: T11
**Reuses**: request helper from T10
**Requirement**: TJ-06

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Sends correct update body against a fake server
- [ ] Transient vs. permanent error distinction matches T12's pattern
- [ ] Gate passes: `go test ./internal/jira/...`
- [ ] Test count: ≥3

**Tests**: unit
**Gate**: quick

---

### T14: Implement `DeleteWorklog`

**What**: `(c *Client) DeleteWorklog(ctx, issueKey, worklogID string) error` — `DELETE .../worklog/{id}`.
**Where**: `to-jira/internal/jira/worklog.go` (append)
**Depends on**: T11
**Reuses**: request helper from T10
**Requirement**: TJ-10

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Successful delete against a fake server returns `nil`
- [ ] Transient vs. permanent error distinction matches T12's pattern
- [ ] Gate passes: `go test ./internal/jira/...`
- [ ] Test count: ≥3

**Tests**: unit
**Gate**: quick

---

### T15: Implement `WarnIfTokenExpiringSoon`

**What**: `(c *Client) WarnIfTokenExpiringSoon(configuredExpiry *time.Time, logger *slog.Logger)` — logs a warning within 14 days of the configured date, an informational note if unset (TJ-15).
**Where**: `to-jira/internal/jira/tokenexpiry.go` (+ `tokenexpiry_test.go`)
**Depends on**: T10
**Reuses**: nothing existing
**Requirement**: TJ-15

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Warns when expiry is ≤14 days away
- [ ] Logs an informational (non-warning) note when `configuredExpiry` is `nil`
- [ ] No log at all when expiry is comfortably far away
- [ ] Gate passes: `go test ./internal/jira/...`
- [ ] Test count: ≥3

**Tests**: unit
**Gate**: quick

---

### T16: Implement `sync.Processor.Process`

**What**: Orchestrates TJ-02…TJ-09 — validate format, running-entry skip, `FindWorklogByTogglID` → create-or-update, dry-run branching, OTel span + counter emission, `Result{HTTPStatus, Outcome}`.
**Where**: `to-jira/internal/sync/process.go` (+ `process_test.go`)
**Depends on**: T7, T8, T12, T13, T4
**Reuses**: `toggl.ParseDescription`/`NormalizeEntry`, `jira.Client` (behind an interface for test faking), `telemetry.Metrics`
**Requirement**: TJ-02, TJ-03, TJ-04, TJ-05, TJ-06, TJ-07, TJ-08, TJ-09

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Every `Outcome` value reachable via a fake `jira.Client`: `created`, `updated`, `skipped_invalid`, `skipped_running`, `transient_error`
- [ ] Dry-run mode reaches the decision point but never calls the fake client's write methods
- [ ] Correct `HTTPStatus` per design.md's Error Handling Strategy table for each outcome
- [ ] Metrics counters incremented on the correct outcomes
- [ ] Gate passes: `go test ./internal/sync/...`
- [ ] Test count: ≥10 (1:1 to the ACs above plus dry-run)

**Tests**: unit
**Gate**: quick

---

### T17: Implement `sync.Processor.ProcessDelete`

**What**: Orchestrates TJ-10…TJ-13 — derive issue key from the delete envelope (both hypothesized shapes), `FindWorklogByTogglID` → delete-or-noop, dry-run branching, telemetry, `Result`.
**Where**: `to-jira/internal/sync/process_delete.go` (+ `process_delete_test.go`)
**Depends on**: T8, T14, T16
**Reuses**: `toggl.NormalizeEntry`, `jira.Client` fake from T16
**Requirement**: TJ-10, TJ-11, TJ-12, TJ-13

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Derivable-payload delete → `deleted` outcome, correct JIRA call
- [ ] No-matching-worklog delete → `noop` outcome, 200, no error
- [ ] Non-derivable payload (delete shape B) → `unsupported_delete` outcome, 200, `validation_errors_total` incremented
- [ ] Dry-run mode reaches the lookup but never calls delete
- [ ] Gate passes: `go test ./internal/sync/...`
- [ ] Test count: ≥8

**Tests**: unit
**Gate**: quick

---

### T18: Implement `internal/webhook` handler

**What**: `NewHandler(secret string, p *sync.Processor, logger) *Handler`, `(h *Handler) Receive(c *gin.Context)` — raw-body capture (`io.ReadAll`, NOT `ShouldBindJSON`, per design.md's flagged gotcha) → HMAC verify → `toggl.ParseEnvelope` → dispatch to `Process`/`ProcessDelete` by `EventMetadata.RequestType` → map `Result` to gin response.
**Where**: `to-jira/internal/webhook/handler.go` (+ `handler_test.go`)
**Depends on**: T16, T17, T6
**Reuses**: `sync.Processor`, `toggl.ParseEnvelope`
**Requirement**: TJ-01 + integration of TJ-02…TJ-13

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Missing/invalid/malformed signature → 401, `sync.Processor` never invoked (verify via a spy)
- [ ] Valid signature + `created`/`updated`/`deleted` events dispatch to the correct `sync` method
- [ ] Unrecognized event type → 200, no dispatch
- [ ] Full happy-path + edge + error paths per the coverage matrix
- [ ] Gate passes: `go test ./internal/webhook/... && go build ./...`
- [ ] Test count: ≥10

**Tests**: unit (httptest)
**Gate**: full

---

### T19: Implement `internal/shared/di`

**What**: Staged `BuildDependencies(cfg *config.Config) (*Dependency, error)` — `buildTelemetry`, `buildClients` (jira.Client), `buildProcessor` (sync.Processor), `buildHandlers` (webhook.Handler) — mirroring dinherim/applyr's staged builder, minus `buildDBs`/`buildRepositories`.
**Where**: `to-jira/internal/shared/di/dependency.go` (+ `dependency_test.go`)
**Depends on**: T18, T4
**Reuses**: `dinherim/internal/shared/di/dependency.go` staged pattern
**Requirement**: N/A (infra)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `BuildDependencies` wires every component with no nil fields on success
- [ ] A config validation failure surfaces as an error, not a panic
- [ ] Gate passes: `go build ./... && go vet ./... && go test ./...`
- [ ] Test count: ≥2

**Tests**: unit
**Gate**: full

---

### T20: Implement `internal/routes`

**What**: Registers `POST /webhooks/toggl` → `webhook.Handler.Receive` on the `v1` route group (per applyr's AD-002 fix — register on the group carrying the middleware chain, not on `app` directly).
**Where**: `to-jira/internal/routes/routes.go`
**Depends on**: T19
**Reuses**: `applyr/internal/routes/routes.go` pattern
**Requirement**: N/A (infra, enables TJ-01)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] Route registered on the correct group
- [ ] `go build ./...` passes

**Tests**: none
**Gate**: build

---

### T21: Implement `main.go` composition root

**What**: Load config → `telemetry.Initialize` → `di.BuildDependencies` → register routes → start server → graceful shutdown (server + telemetry).
**Where**: `to-jira/main.go`
**Depends on**: T20
**Reuses**: dinherim/applyr's `main.go` `buildApp()` composition pattern
**Requirement**: N/A (integration point for all TJ-* requirements)

**Tools**:
- MCP: NONE
- Skill: NONE

**Done when**:
- [ ] `go build ./...` produces a working binary
- [ ] Manual smoke check: binary starts, logs a startup message, exits cleanly on SIGTERM (no automated test — this is the composition root, covered by the `none` matrix row)
- [ ] Full-repo gate passes: `go build ./... && go vet ./... && go test ./...`

**Tests**: none
**Gate**: build

---

## Phase Execution Map

```
Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5

Phase 1:  T1 ──→ T2 ──→ T3 ──→ T4 ──→ T5
Phase 2:  T6 ──→ T7 ──→ T8
Phase 3:  T9 ──→ T10 ──→ T11 ──→ T12 ──→ T13 ──→ T14 ──→ T15
Phase 4:  T16 ──→ T17
Phase 5:  T18 ──→ T19 ──→ T20 ──→ T21
```

Execution is strictly sequential — there is no intra-phase parallelism.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: Module skeleton | 3 files, no logic | ✅ Granular |
| T2: Port logger | 1 component | ✅ Granular |
| T3: Add config | 1 component | ✅ Granular |
| T4: Add telemetry | 1 component | ✅ Granular |
| T5: Port server | 1 component | ✅ Granular |
| T6: Envelope types + ParseEnvelope | 1 function + supporting types | ✅ Granular |
| T7: ParseDescription | 1 function | ✅ Granular |
| T8: NormalizeEntry | 1 function | ✅ Granular |
| T9: ADF types + build/extract | 2 tightly-coupled functions, same file | ✅ Granular (cohesive) |
| T10: Client + auth helper | 1 component | ✅ Granular |
| T11: FindWorklogByTogglID | 1 function | ✅ Granular |
| T12: CreateWorklog | 1 function | ✅ Granular |
| T13: UpdateWorklog | 1 function | ✅ Granular |
| T14: DeleteWorklog | 1 function | ✅ Granular |
| T15: WarnIfTokenExpiringSoon | 1 function | ✅ Granular |
| T16: Process | 1 function (orchestration) | ✅ Granular |
| T17: ProcessDelete | 1 function (orchestration) | ✅ Granular |
| T18: webhook.Handler | 1 component | ✅ Granular |
| T19: di.BuildDependencies | 1 component | ✅ Granular |
| T20: routes | 1 file change | ✅ Granular |
| T21: main.go | 1 file, composition only | ✅ Granular |

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| --- | --- | --- | --- |
| T1 | None | (start) | ✅ Match |
| T2 | T1 | T1→T2 | ✅ Match |
| T3 | T1 | T2→T3 | ✅ Match* |
| T4 | T3 | T3→T4 | ✅ Match |
| T5 | T3 | T4→T5 | ✅ Match* |
| T6 | T1 | T5→T6 (phase order) | ✅ Match* |
| T7 | T6 | T6→T7 | ✅ Match |
| T8 | T6 | T7→T8 | ✅ Match* |
| T9 | T1 | T8→T9 (phase order) | ✅ Match* |
| T10 | T9 | T9→T10 | ✅ Match |
| T11 | T10 | T10→T11 | ✅ Match |
| T12 | T11 | T11→T12 | ✅ Match |
| T13 | T11 | T12→T13 | ✅ Match* |
| T14 | T11 | T13→T14 | ✅ Match* |
| T15 | T10 | T14→T15 | ✅ Match* |
| T16 | T7, T8, T12, T13, T4 | T15→T16 (phase order) | ✅ Match* |
| T17 | T8, T14, T16 | T16→T17 | ✅ Match |
| T18 | T16, T17, T6 | T17→T18 (phase order) | ✅ Match* |
| T19 | T18, T4 | T18→T19 | ✅ Match |
| T20 | T19 | T19→T20 | ✅ Match |
| T21 | T20 | T20→T21 | ✅ Match |

\* Within a phase, the execution diagram shows strict linear order (matching "tasks within a phase execute in order"); the task body's `Depends on` may name an earlier task in the same or a prior phase rather than the immediately-preceding one — both are satisfied since dependencies only ever point backward or to the same phase, never forward, consistent with the linear execution order shown.

No forward dependencies found. No mismatches.

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1 | scaffolding | none | none | ✅ OK |
| T2 | logger | none | none | ✅ OK |
| T3 | config | unit | unit | ✅ OK |
| T4 | telemetry | none | none | ✅ OK |
| T5 | server | none | none | ✅ OK |
| T6 | toggl | unit | unit | ✅ OK |
| T7 | toggl | unit | unit | ✅ OK |
| T8 | toggl | unit | unit | ✅ OK |
| T9 | jira | unit | unit | ✅ OK |
| T10 | jira | unit | unit | ✅ OK |
| T11 | jira | unit | unit | ✅ OK |
| T12 | jira | unit | unit | ✅ OK |
| T13 | jira | unit | unit | ✅ OK |
| T14 | jira | unit | unit | ✅ OK |
| T15 | jira | unit | unit | ✅ OK |
| T16 | sync | unit | unit | ✅ OK |
| T17 | sync | unit | unit | ✅ OK |
| T18 | webhook | unit | unit | ✅ OK |
| T19 | di | none | none | ✅ OK |
| T20 | routes | none | none | ✅ OK |
| T21 | main | none | none | ✅ OK |

No violations. No task defers its tests to a later task.

---

## Tips

- **Phases are ordered** — Each phase completes before the next; tasks run in order within a phase
- **Reuses = Token saver** — Always reference existing code
- **Tools per task** — MCPs and Skills prevent wrong approaches
- **Dependencies are gates** — Clear what blocks what
- **Done when = Testable** — If you can't verify it, rewrite it
- **Requirement ID = Traceable** — Every task traces back to a spec requirement
- **One commit per task** — Plan the commit message format in advance
