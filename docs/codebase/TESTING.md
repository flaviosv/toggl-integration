# Testing Infrastructure

## Test Frameworks

- **Unit:** stdlib `testing` only — no third-party assertion or mocking library. `httptest` (`httptest.Server`, `httptest.NewRecorder`) for HTTP-boundary tests.
- **Integration/E2E:** none — no live network calls are made in any test; hand-written fakes stand in for external dependencies.
- **Coverage tool:** none configured (`go test -cover` is available via stdlib but no target/threshold is enforced).

## Test Organization

- **Location:** co-located with source, one `<file>_test.go` per `<file>.go` in the same package (Go convention) — e.g. `internal/jira/worklog.go` ↔ `internal/jira/worklog_test.go`.
- **Naming:** `Test<Subject>_<Scenario>`, scenario-specific enough to read as a spec — e.g. `TestNormalizeEntry_DeleteShapeB_DescriptionNil`, `TestDo_ReturnsNon2xxWithoutError`.
- **Structure:** table-driven where a function has several input/output cases (`TestParseDescription`), one function per behavior otherwise. Test-only support types (fakes) live in their own `*_test.go` file per package — `internal/sync/fake_client_test.go`, and a package-local equivalent in `internal/webhook`.

## Testing Patterns

- **Unit — pure functions** (`toggl.ParseDescription`, `toggl.NormalizeEntry`, `jira.BuildComment`/`ExtractTogglID`): direct input/output assertions, table-driven for the branchy ones.
- **Unit — HTTP client boundary** (`internal/jira`): `httptest.Server` stands in for the JIRA API; tests assert on request shape (path, auth header, body) and on response handling (status classification, decode).
- **Unit — orchestration** (`internal/sync`): a hand-written `fakeJiraClient` implementing the `sync.JiraClient` interface, with call counts and last-call-argument capture, asserts both the outcome (`Result.Outcome`/`HTTPStatus`) and that the right JIRA method was (or wasn't) called.
- **Unit — HTTP handler** (`internal/webhook`): `httptest.NewRecorder` + a real `gin.Context`, backed by the same kind of fake JIRA client (via a real `sync.Processor`), asserting HTTP status and that JIRA methods are never called on an authentication failure.
- **Hard project constraint:** no live network calls in any test — stated explicitly in `internal/sync/fake_client_test.go` and `internal/webhook/handler_test.go` comments.

## Test Execution

- **Commands:** run from `to-jira/` — `go test ./...` (all), `go test ./internal/<pkg>/...` (one package).
- **Configuration:** none beyond stdlib defaults; no `go.work`, no test tags, no `testdata/` fixtures observed.

## Coverage Targets

No formal coverage percentage target or enforcement tool is configured. The de facto target, per the Test Coverage Matrix below (from `.specs/features/to-jira/tasks.md`), is "all branches" for every layer with actual business logic, and "none — build gate only" for thin wiring/bootstrap code.

## Test Coverage Matrix

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------- | --------------------- | ----------------- | ------------ |
| `internal/toggl` (parsing/validation) | unit | All branches; malformed formats, running entries, both hypothesized delete-payload shapes | `internal/toggl/*_test.go` | `go test ./internal/toggl/...` |
| `internal/jira` (API client) | unit | All CRUD paths + ADF build/extract round-trip + transient vs. permanent error handling, via `httptest.Server` fakes — no live network calls | `internal/jira/*_test.go` | `go test ./internal/jira/...` |
| `internal/sync` (orchestration) | unit | Every `Result.Outcome` value reachable (`created`, `updated`, `skipped_invalid`, `skipped_running`, `deleted`, `noop`, `unsupported_delete`, `transient_error`) via a fake `jira.Client` | `internal/sync/*_test.go` | `go test ./internal/sync/...` |
| `internal/webhook` (HTTP handler) | unit (`httptest`) | Signature verification (valid/invalid/missing) + dispatch + HTTP status mapping; happy + edge + error paths | `internal/webhook/*_test.go` | `go test ./internal/webhook/...` |
| `internal/shared/config` | unit | Required-field validation — missing/invalid env vars rejected at load | `internal/shared/config/*_test.go` | `go test ./internal/shared/config/...` |
| `internal/shared/{logger,telemetry,server,di}`, `internal/routes`, `main.go` | none | Thin wiring/bootstrap, no branching logic — build gate only | — | `go build ./...` |

## Parallelism Assessment

| Test Type | Parallel-Safe? | Isolation Model | Evidence |
| --------- | --------------- | ---------------- | -------- |
| All unit tests | Yes | Each test constructs its own fake/client/server instance; no shared mutable package state or database observed. `t.Parallel()` is used explicitly in `internal/jira/client_test.go`. | No global state, no shared `httptest.Server` reused across test functions, fakes are per-test-function locals |

## Gate Check Commands

| Gate Level | When to Use | Command |
| ---------- | ------------ | ------- |
| Quick | After a task touching one package with unit tests | `go test ./internal/<pkg>/...` |
| Full | After a task wiring multiple packages together (webhook, di, main) | `go build ./... && go vet ./... && go test ./...` |
| Build | After each phase completes | `go build ./... && go vet ./... && go test ./...` |

No lint tool or CI-enforced gate exists yet — `go vet` is included as a free stdlib check, not a CI dependency (the project explicitly defers CI, see `docs/codebase/PROJECT.md`).
