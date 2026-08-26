# Testing Infrastructure

Two independent test suites — Go stdlib `testing` for `to-jira`, Node's built-in `node --test` for
`mcp` — with no shared tooling or fixtures.

## Test Frameworks

- **`to-jira` Unit:** stdlib `testing` only — no third-party assertion or mocking library. `httptest` (`httptest.Server`, `httptest.NewRecorder`) for HTTP-boundary tests.
- **`to-jira` Integration/E2E:** none — no live network calls are made in any test; hand-written fakes stand in for external dependencies.
- **`mcp` Unit/Integration:** Node's built-in `node --test` (no Jest/Vitest/Mocha) against compiled `dist/**/*.test.js`. A shared local `http.Server` fake stands in for the Toggl API (`mcp/src/tools/test-harness.ts`'s `startFakeToggl`); `InMemoryTransport` stands in for stdio for in-process MCP client/server tests (`connectToolClient`). One file (`index.test.ts`) spawns the real compiled server as a subprocess to test process-level behavior (bad-config exit codes, MCP handshake over real stdio).
- **Coverage tool:** none configured in either package (`go test -cover` is available but unused; no `c8`/`nyc`/`--experimental-test-coverage` wired for `mcp`).

## Test Organization

- **`to-jira` Location:** co-located with source, one `<file>_test.go` per `<file>.go` in the same package (Go convention) — e.g. `internal/jira/worklog.go` ↔ `internal/jira/worklog_test.go`.
- **`to-jira` Naming:** `Test<Subject>_<Scenario>`, scenario-specific enough to read as a spec — e.g. `TestNormalizeEntry_DeleteShapeB_DescriptionNil`, `TestDo_ReturnsNon2xxWithoutError`.
- **`to-jira` Structure:** table-driven where a function has several input/output cases (`TestParseDescription`), one function per behavior otherwise. Test-only support types (fakes) live in their own `*_test.go` file per package — `internal/sync/fake_client_test.go`, and a package-local equivalent in `internal/webhook`.
- **`mcp` Location:** co-located with source, one `<file>.test.ts` per `<file>.ts` — e.g. `config.ts` ↔ `config.test.ts`. Shared test-only support lives in `src/tools/test-harness.ts` (fake HTTP server, in-memory MCP client/server wiring, tmp-cache-path helpers).
- **`mcp` Naming:** plain-English `test(...)` descriptions (Node's `node:test` API), not a `Test<Subject>_<Scenario>` convention — e.g. `"missing TOGGL_API_TOKEN: non-zero exit, stderr names the variable, zero bytes on stdout"`.
- **`mcp` Structure:** one `test(...)` per behavior/branch; data-driven cases collapsed into a loop where the assertion shape repeats (e.g. config's invalid-value cases).

## Testing Patterns

- **`to-jira` Unit — pure functions** (`toggl.ParseDescription`, `toggl.NormalizeEntry`, `jira.BuildComment`/`ExtractTogglID`): direct input/output assertions, table-driven for the branchy ones.
- **`to-jira` Unit — HTTP client boundary** (`internal/jira`): `httptest.Server` stands in for the JIRA API; tests assert on request shape (path, auth header, body) and on response handling (status classification, decode).
- **`to-jira` Unit — orchestration** (`internal/sync`): a hand-written `fakeJiraClient` implementing the `sync.JiraClient` interface, with call counts and last-call-argument capture, asserts both the outcome (`Result.Outcome`/`HTTPStatus`) and that the right JIRA method was (or wasn't) called.
- **`to-jira` Unit — HTTP handler** (`internal/webhook`): `httptest.NewRecorder` + a real `gin.Context`, backed by the same kind of fake JIRA client (via a real `sync.Processor`), asserting HTTP status and that JIRA methods are never called on an authentication failure.
- **`to-jira` Hard project constraint:** no live network calls in any test — stated explicitly in `internal/sync/fake_client_test.go` and `internal/webhook/handler_test.go` comments.
- **`mcp` Unit — pure functions** (`curate.toCuratedEntry`, `schemas.dateOrTimestamp`/`positiveId`, `logger.log`): direct input/output assertions.
- **`mcp` Unit — HTTP client boundary** (`toggl/client.ts`): a local `http.Server` fake stands in for the Toggl API; tests assert on request shape and on response handling (status classification, JSON/non-JSON/empty body parsing, timeout).
- **`mcp` Unit — cache layer** (`cache/project-cache.ts`): real temp-directory disk I/O (`fs/promises` against `os.tmpdir()`-rooted paths), asserting TTL freshness/staleness, stale-cache fallback on refetch failure, and best-effort write-failure degrade.
- **`mcp` Integration — tool call** (`tools/list-time-entries.test.ts`): `connectToolClient` wires a real `McpServer` + `Client` over `InMemoryTransport` (no stdio/subprocess) against a fake Toggl HTTP server, exercising the full validate → fetch → filter → resolve-project → curate path end to end.
- **`mcp` Integration — process boundary** (`index.test.ts`): spawns the real compiled `dist/index.js` as an OS subprocess to verify config-error exit codes/stderr and one real stdio MCP handshake — the one file in the suite doing real process I/O.
- **`mcp` Hard project constraint:** no live Toggl network calls in any test — every HTTP interaction goes through `test-harness.ts`'s fake server.

## Test Execution

- **`to-jira` Commands:** run from `to-jira/` — `go test ./...` (all), `go test ./internal/<pkg>/...` (one package).
- **`to-jira` Configuration:** none beyond stdlib defaults; no `go.work`, no test tags, no `testdata/` fixtures observed.
- **`mcp` Commands:** run from `mcp/` — `npm test` (build + all tests), `npm run test:unit` (fast subset: `config`, `tools/**`), `npm run test:integration` (`index.test.ts` only, real subprocess spawn).
- **`mcp` Configuration:** tests run against compiled `dist/**/*.test.js`, not `src/` directly — `npm test`/`test:unit`/`test:integration` all run `tsc` first. No `testdata/` fixtures; fixtures are inline objects per test file.

## Coverage Targets

No formal coverage percentage target or enforcement tool is configured in either package. The de facto target, per the Test Coverage Matrix below (from `.specs/features/to-jira/tasks.md` and `.specs/features/TOGGL-2-time-entries-mcp/tasks.md`), is "all branches" for every layer with actual business logic, and "none — build gate only" for thin wiring/bootstrap code.

## Test Coverage Matrix

**`to-jira`:**

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------- | --------------------- | ----------------- | ------------ |
| `internal/toggl` (parsing/validation) | unit | All branches; malformed formats, running entries, both hypothesized delete-payload shapes | `internal/toggl/*_test.go` | `go test ./internal/toggl/...` |
| `internal/jira` (API client) | unit | All CRUD paths + ADF build/extract round-trip + transient vs. permanent error handling, via `httptest.Server` fakes — no live network calls | `internal/jira/*_test.go` | `go test ./internal/jira/...` |
| `internal/sync` (orchestration) | unit | Every `Result.Outcome` value reachable (`created`, `updated`, `skipped_invalid`, `skipped_running`, `deleted`, `noop`, `unsupported_delete`, `transient_error`) via a fake `jira.Client` | `internal/sync/*_test.go` | `go test ./internal/sync/...` |
| `internal/webhook` (HTTP handler) | unit (`httptest`) | Signature verification (valid/invalid/missing) + dispatch + HTTP status mapping; happy + edge + error paths | `internal/webhook/*_test.go` | `go test ./internal/webhook/...` |
| `internal/shared/config` | unit | Required-field validation — missing/invalid env vars rejected at load | `internal/shared/config/*_test.go` | `go test ./internal/shared/config/...` |
| `internal/shared/{logger,telemetry,server,di}`, `internal/routes`, `main.go` | none | Thin wiring/bootstrap, no branching logic — build gate only | — | `go build ./...` |

**`mcp`:**

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------- | --------------------- | ----------------- | ------------ |
| `src/config.ts` | unit | Every invalid/missing-var combination, whitespace-only values, cache-path fallback | `src/config.test.ts` | `node --test dist/config.test.js` |
| `src/toggl/client.ts` | unit | Status classification, timeout, JSON/non-JSON/empty body parsing — via a fake HTTP server, no live network calls | `src/toggl/client.test.ts` | `node --test dist/toggl/client.test.js` |
| `src/cache/project-cache.ts` | unit | TTL freshness/staleness (incl. `>=` boundary), stale-cache fallback, malformed-cache-file handling, write-failure degrade | `src/cache/project-cache.test.ts` | `node --test dist/cache/project-cache.test.js` |
| `src/time-entries/curate.ts` | unit | Defensive-default fields, `project_name` vs. cache-lookup precedence | `src/time-entries/curate.test.ts` | `node --test dist/time-entries/curate.test.js` |
| `src/tools/list-time-entries.ts` | integration (`InMemoryTransport`) | Full tool call incl. both Toggl-call and project-fetch failure paths, `stale_cache` warning surfacing | `src/tools/list-time-entries.test.ts` | `node --test dist/tools/list-time-entries.test.js` |
| `src/errors.ts`, `src/tools/schemas.ts`, `src/logger.ts` | unit | Pure-function branch coverage | `src/*.test.ts` | `npm run test:unit` |
| `src/index.ts` | integration (real subprocess) | Config-error exit codes/stderr, one real stdio MCP handshake | `src/index.test.ts` | `npm run test:integration` |

## Parallelism Assessment

| Test Type | Parallel-Safe? | Isolation Model | Evidence |
| --------- | --------------- | ---------------- | -------- |
| `to-jira` unit tests | Yes | Each test constructs its own fake/client/server instance; no shared mutable package state or database observed. `t.Parallel()` is used explicitly in `internal/jira/client_test.go`. | No global state, no shared `httptest.Server` reused across test functions, fakes are per-test-function locals |
| `mcp` unit/integration tests | Yes, with a known leak | Each test starts its own fake HTTP server / temp cache directory / in-memory transport pair; no shared mutable module state. Not fully clean: several tests (`list-time-entries.test.ts`, `index.test.ts`) create a temp dir via `mkdtempSync` and don't remove it in every case, leaking directories under the OS temp folder across runs (flagged, not blocking — see `docs/codebase/CONCERNS.md`). | Per-test `startFakeToggl`/`connectToolClient`/`makeTmpCachePath` calls; no test reuses another test's server/client instance |

## Gate Check Commands

**`to-jira`:**

| Gate Level | When to Use | Command |
| ---------- | ------------ | ------- |
| Quick | After a task touching one package with unit tests | `go test ./internal/<pkg>/...` |
| Full | After a task wiring multiple packages together (webhook, di, main) | `go build ./... && go vet ./... && go test ./...` |
| Build | After each phase completes | `go build ./... && go vet ./... && go test ./...` |

No lint tool or CI-enforced gate exists yet — `go vet` is included as a free stdlib check, not a CI dependency (the project explicitly defers CI, see `docs/codebase/PROJECT.md`).

**`mcp`:**

| Gate Level | When to Use | Command |
| ---------- | ------------ | ------- |
| Quick | After a task touching one module with unit tests | `npm run test:unit` (run from `mcp/`) |
| Full | After a task touching tool registration or the entrypoint | `npm test` (build + full suite, includes the subprocess integration test) |
| Build | After each phase completes | `npm run build && npm test` |

No lint tool or CI-enforced gate exists yet for `mcp` either.
