# Fix Review Plan — PR #1 (flaviosv/toggl-integration)

Fetched fresh via GraphQL `reviewThreads` on 2026-08-22. 47 unresolved threads, all single-comment
(the original review finding, no further discussion). No pagination cap hit (47 < 100).

Validated against current code AND against `.specs/features/to-jira/design.md` +
`.specs/STATE.md` (AD-001..AD-003, Q8 decision) — several findings that look like bugs in isolation
are actually documented, deliberate architecture decisions. Those are pushed back with citations.

## Pushback (no code change — reply + resolve only)

- **Thread 1 (A1, Critical, process.go:85)** — claims permanent JIRA failures (404/400) should map
  to a different, non-retriable outcome instead of the same 502/transient_error as real transient
  failures. **Does not hold up**: `design.md`'s Error Handling Strategy table explicitly documents
  "JIRA call fails permanently (404 unknown issue, 400) | ... same non-2xx ... accepted per spec's
  no-allowlist decision" and the Risks & Concerns table repeats this as an accepted, understood
  trade-off (Q8 decision, no further mitigation needed). This is deliberate, not a bug.
- **Thread 7 (A7, Low, dependency.go:26)** — flags `newMetrics` as a package-level mutable global
  ("hidden dependency"), recommending an explicit constructor parameter. **Does not hold up as
  something to change**: `design.md` explicitly says `di.BuildDependencies` is a "copy pattern" of
  `dinherim/applyr`'s staged builder, and the file's own doc comment identifies this exact
  var-swap-for-testability idiom as mirroring that prior project's `var dbNew = db.New`. This is a
  deliberate, documented architectural choice to replicate a known-working pattern, not an
  accidental anti-pattern — low severity, intentional, consistent with the project's own "copy
  verbatim" decision.
- **Thread 13 (Q6, Low, dependency.go:2)** — claims the package doc's references to
  "dinherim/applyr" are fabricated citations that "don't exist anywhere in this repository."
  **Does not hold up**: `design.md` explicitly states "Approach 2 (confirmed): reuse dinherim/applyr's
  infra plumbing (`config`, `logger`, `di`, `server`, `routes`) verbatim" and separately documents
  the `di.BuildDependencies` staged-builder copy pattern in the exact terms the code comment uses
  (drop `buildDBs`/`buildRepositories`, add `buildClients`/`buildTelemetry`). The citation is
  accurate and traceable within this repo's own spec docs.
- **Thread 40 (M3, Medium, fake_client_test.go:12)** — recommends hoisting `sync.fakeJiraClient` /
  `webhook.fakeJiraClient` into a new shared `internal/jira/jiratest` package to eliminate
  duplication across the two test packages. **Doesn't hold up as worth the added complexity**:
  thread 39 (M2, applied — see Cluster: webhook) already eliminates the more significant
  in-package duplication (two nearly-identical fakes in the same file). What remains after that is
  a single small test double duplicated once across two packages — a "rule of two," not a "rule of
  three." Go idiomatically keeps small package-local test doubles even with light duplication,
  specifically to avoid coupling two packages' test suites through a shared test-only package for
  ~40 lines of stub code. Not worth a new package for this codebase's size.
- **Thread 41 (M4, Medium, dependency_test.go:39)** — flags `TestBuildDependencies_WiresAllComponentsWithNoNilFields`
  reaching into unexported `d.metrics`/`d.tracer`/`d.clients.jira`, recommending verification via
  public API / observable side effects instead. **Doesn't hold up**: this is a same-package
  (`package di`, not `di_test`) whitebox test, an accepted, common Go idiom for verifying a DI
  container's internal wiring — exactly the case where there's no cheaper alternative (a nil field
  silently stored by a DI stage won't surface as a failure until much later; asserting through
  `d.Processor != nil` doesn't prove `d.Processor.tracer`/`d.Processor.metrics` are non-nil, since
  those are just stored, not used, at construction time). The suggested alternative (asserting via
  emitted spans/metrics) requires disproportionate new scaffolding for a "Medium" internal-coupling
  concern. Consistent with the project's own copied DI/testability conventions (see A7 above).

## Cluster: internal/webhook/handler.go, internal/webhook/handler_test.go, internal/sync/process.go, internal/sync/process_delete.go, internal/sync/process_delete_test.go, internal/toggl/normalize.go, internal/toggl/normalize_test.go

Work through in this exact order — later steps depend on earlier ones leaving a compiling, passing
state.

1. **Thread 11 (Q4, apply-as-directed, process.go:72)** — `logger.FromContext(ctx).Warn("sync:
   invalid description format", ..., "description", e.Description)` logs the raw, potentially
   business-sensitive Toggl description at WARN. Fix: log only `len(e.Description)` (or similar) at
   WARN; drop the raw text from this log line entirely (no DEBUG-level raw-text logging needed —
   keep this minimal). Test: none exists asserting the previous log content, so no update needed;
   optionally add/adjust a `process_test.go` assertion that the WARN log line does NOT contain the
   raw description substring, if convenient — not required if it complicates things.

2. **Thread 10 (Q3, auto-fix, process.go:83, also process_delete.go:39,53)** — 5 near-identical
   `logger.Error(...); p.metrics.JiraAPIErrors.Add(ctx, 1); return Result{HTTPStatus: 502,
   Outcome: OutcomeTransientError}` blocks (process.go:83-86,106-109,120-123 and
   process_delete.go:39-41,53-55). Extract a helper, e.g.
   `func (p *Processor) jiraError(ctx context.Context, op, togglID, issueKey string, err error) Result`
   that logs (`"sync: "+op+" failed"` or similar per-callsite message), increments
   `JiraAPIErrors`, and returns `Result{HTTPStatus: http.StatusBadGateway, Outcome:
   OutcomeTransientError}`. Call it from all 5 sites. **Do NOT add any errors.As/permanent-vs-transient
   branching** — thread 1 established that uniform 502/transient_error for ALL JIRA failures
   (transient or permanent) is the deliberate, documented behavior (see Pushback above). This is a
   pure deduplication, no behavior change. Existing tests (`process_test.go`,
   `process_delete_test.go`) already assert on the resulting `Result`/metric and should still pass
   unchanged — run them to confirm.

3. **Thread 24 (R1, auto-fix, process_delete.go:18)** — `ProcessDelete` starts its own
   `"sync.ProcessDelete"` span but never calls `span.SetAttributes`, unlike `Process` (which does
   `span.SetAttributes(attribute.String("toggl.id", e.TogglID))` right after normalizing, at
   process.go:68). Fix: after normalizing the entry in `ProcessDelete` (once `e.TogglID` is
   available, right after the `toggl.NormalizeEntry` call), add the same
   `span.SetAttributes(attribute.String("toggl.id", e.TogglID))`. Test: add
   `TestProcessDelete_Success_EmitsSpanTaggedWithTogglID` in `process_delete_test.go`, mirroring
   `TestProcess_Success_EmitsSpanTaggedWithTogglID` in `process_test.go` (uses
   `newRecordingTracer(t)` from `metrics_test.go`, asserts span name `"sync.ProcessDelete"` and the
   `toggl.id` attribute).

4. **Thread 9 (Q2, auto-fix, normalize.go:28)** — `NormalizeEntry` returns `(Event, bool)` where
   `ok` duplicates `Event.HasStopped`; every production caller (`webhook/handler.go:71`,
   `sync/process_delete.go:28`) already discards it via `_`. Fix: change signature to
   `func NormalizeEntry(p TimeEntryPayload) Event` (drop the bool return). Update both call sites:
   `webhook/handler.go:71` (`event, _ := toggl.NormalizeEntry(payload)` → `event :=
   toggl.NormalizeEntry(payload)`) and `sync/process_delete.go:28` (`e, _ :=
   toggl.NormalizeEntry(payload)` → `e := toggl.NormalizeEntry(payload)`). Update
   `normalize_test.go`: every test currently asserting on the `ok` return value
   (`TestNormalizeEntry_CompleteEntry`, `TestNormalizeEntry_RunningEntry`,
   `TestNormalizeEntry_DeleteShapeA_DescriptionPresentNoDuration`,
   `TestNormalizeEntry_DeleteShapeB_DescriptionNil`) must instead assert on `e.HasStopped` (for the
   running-vs-stopped cases) — the "DeleteShapeA/B" tests don't need `ok` at all, they only assert
   on `e.Description`, so just drop the `ok` variable there. Keep the same table/case structure,
   only change what's asserted.

5. **Thread 32 (V8, auto-fix, normalize_test.go:48)** — the running-entry check is `*p.Duration <
   0`, so `Duration == 0` is a valid boundary (`HasStopped` should be `true`), but no test covers
   exactly `Duration: 0`. Add a case to `TestNormalizeEntry_RunningEntry`'s table (or a new small
   test) with `Duration: ptr(int64(0))`, `Start`/`Stop` both set, asserting `e.HasStopped == true`
   (this is a *positive* case, not a "running" case — name it accordingly, e.g. move it into
   `TestNormalizeEntry_CompleteEntry`-style assertion or a new `TestNormalizeEntry_ZeroDuration`
   test asserting `HasStopped == true` and `Duration == 0`).

6. **Threads 2 and 18 (A2 + H1, auto-fix — duplicate findings, same fix, handler.go)** —
   `logger.WithLogger` exists specifically so `logger.FromContext(ctx)` can retrieve a per-request
   logger, but no production caller ever calls it, so every `logger.FromContext(ctx)` call in
   `sync/process.go`/`process_delete.go` (9 call sites) silently falls back to `slog.Default()`
   instead of the JSON-handler logger built in `main.go`. Fix: in `webhook.Handler.Receive`, right
   after signature verification succeeds (before dispatching to `Process`/`ProcessDelete`), do
   `ctx := logger.WithLogger(c.Request.Context(), h.logger)` and pass `ctx` (not
   `c.Request.Context()`) into `h.p.Process(ctx, event)` and `h.p.ProcessDelete(ctx, env)`. Import
   `"github.com/flaviosv/toggl-integration/to-jira/internal/shared/logger"` in handler.go. No
   change needed to `logger.go` itself — `WithLogger`/`FromContext` are already correct, just
   unused until this wiring lands. Test: no existing handler_test.go test asserts on log output: a
   lightweight addition is welcome but not required (existing tests use `io.Discard` loggers) —
   skip adding a new test for this specific wiring if it would require nontrivial new scaffolding;
   note that as the reason if skipped.

7. **Threads 3, 8, 16, 19, 20 (A3 + Q1 + P2 + H2 + S1, auto-fix — 5 duplicate findings on the same
   two lines, handler.go:48)** — `body, _ := io.ReadAll(c.Request.Body)` (a) ignores the read
   error (a truncated/aborted body is silently treated as a signature-verification failure with no
   distinguishing log line) and (b) has no size cap, so an unauthenticated caller can send an
   arbitrarily large body before HMAC verification even runs. Fix, combined:
   ```go
   const maxWebhookBodyBytes = 1 << 20 // 1 MiB, well above any real Toggl payload

   body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBodyBytes))
   if err != nil {
       h.logger.Warn("webhook: failed to read request body", "error", err)
       c.Status(http.StatusBadRequest)
       return
   }
   ```
   (adjust the exact status/constant placement to fit the file's existing const block at the top).
   This single change resolves all 5 threads (3, 8, 16, 19, 20). Test: add
   `TestReceive_BodyExceedsMaxSize_Returns400` (or similar) to `handler_test.go` — POST a body
   larger than `maxWebhookBodyBytes` and assert a non-401/non-200 (400/413) response; a dedicated
   "read error" test is not practical with `httptest` (can't easily simulate a genuine mid-read I/O
   error) — the size-cap test exercises the same error-return code path, which is sufficient.

8. **Thread 39 (M2, auto-fix, handler_test.go:32-61 and 285-296)** — `fakeJiraClient` and
   `fakeJiraClientErr` in `handler_test.go` are two near-duplicate `sync.JiraClient` doubles, and
   `newTestHandler` hardcodes a fresh `fakeJiraClient`, forcing
   `TestReceive_JiraLookupFails_ReturnsNon2xx` to re-copy the entire gin/meter/metrics/processor/
   handler/router setup just to swap in `fakeJiraClientErr`. Fix: merge into one fake with
   configurable per-call errors (mirror `sync/fake_client_test.go`'s `fakeJiraClient`, which
   already has `findErr`/`createErr`/`updateErr`/`deleteErr` fields), and change
   `newTestHandler(t *testing.T)` to `newTestHandler(t *testing.T, fake *fakeJiraClient)` so
   callers pass a pre-built double (construct `&fakeJiraClient{}` at existing call sites, and
   `&fakeJiraClient{findErr: &jira.TransientError{Err: context.DeadlineExceeded}}` for
   `TestReceive_JiraLookupFails_ReturnsNon2xx`, which can then use the shared helper and drop its
   duplicated setup). Remove `fakeJiraClientErr` entirely once its one use is migrated. Existing
   tests must keep passing with the parameterized signature.

9. **Thread 36 (Perf P3, auto-fix, handler_test.go:108)** — none of the 9 `TestReceive_*` tests
   call `t.Parallel()`, though each builds its own fresh `gin.Engine`/fake with no shared state
   (`httptest.NewRequest`/`NewRecorder` is in-memory, no real socket I/O). Add `t.Parallel()` as
   the first line of each `TestReceive_*` test function body (do this last, after step 8's
   `newTestHandler` signature change settles, so parallel tests use the final helper shape).

## Cluster: main.go, internal/routes/routes.go, internal/routes/routes_test.go, internal/shared/di/dependency.go, internal/shared/di/dependency_test.go, internal/jira/tokenexpiry.go, internal/jira/tokenexpiry_test.go

Work through in this order:

1. **Thread 6 (A6, auto-fix, dependency.go:35)** — `d.buildOrder` is appended to in all 4 build
   stages but never read/logged/exposed anywhere. Remove the `buildOrder []string` field from
   `Dependency` and the 4 `d.buildOrder = append(...)` lines in `buildTelemetry`/`buildClients`/
   `buildProcessor`/`buildHandlers`. No test references it (confirm via grep before removing) — no
   test changes needed.

2. **Thread 14 (Q7, apply-as-directed, tokenexpiry.go:16)** — `(c *Client)
   WarnIfTokenExpiringSoon(...)` never references `c`, mixing an unrelated concern (config-expiry
   warning) into the worklog-CRUD `Client` type. Fix exactly as directed: make it a standalone
   function `func WarnIfTokenExpiringSoon(configuredExpiry *time.Time, logger *slog.Logger)` in
   package `jira` (drop the `(c *Client)` receiver). Remove `Dependency.WarnIfTokenExpiringSoon` in
   `dependency.go` (the delegate wrapper) entirely. In `main.go`, change
   `deps.WarnIfTokenExpiringSoon(cfg.Jira.APITokenExpires, baseLogger)` to
   `jira.WarnIfTokenExpiringSoon(cfg.Jira.APITokenExpires, baseLogger)` (add the `jira` import to
   main.go; this call no longer needs `deps` at all, so it can even move earlier in `main()`, but
   moving it isn't required — just repoint the call). In `tokenexpiry_test.go`, change every
   `c.WarnIfTokenExpiringSoon(&expiry, logger)` / `c.WarnIfTokenExpiringSoon(nil, logger)` call to
   the standalone `WarnIfTokenExpiringSoon(&expiry, logger)` / `WarnIfTokenExpiringSoon(nil,
   logger)` and drop the now-unused `c := NewClient(...)` lines in each test. In
   `dependency_test.go`, remove `TestDependency_WarnIfTokenExpiringSoon_DelegatesToWiredClient`
   entirely (the behavior it tested no longer belongs to `Dependency` — it's now directly covered
   by `tokenexpiry_test.go`'s own tests) — this is a clean removal with a replacement already in
   place, not a coverage loss.

3. **Thread 31 (V7, auto-fix, tokenexpiry_test.go:17)** — the function's doc comment calls out
   "already past" as a case it warns on, but every test uses a future expiry. Add
   `TestWarnIfTokenExpiringSoon_AlreadyPast_Warns`: `expiry := time.Now().Add(-24 * time.Hour)`,
   assert the log output contains `"level=WARN"` (same style as
   `TestWarnIfTokenExpiringSoon_WarnsWithinWindow`, using the standalone function per step 2).

4. **Thread 5 (A5, auto-fix, main.go:63)** — `server.Run` already applies `shutdownGrace`
   internally to drain in-flight requests before returning; `main.go` then applies a *second*,
   independent `shutdownGrace`-duration timeout to telemetry shutdown afterward, additive to ~20s
   worst case. Fix: give telemetry shutdown its own smaller explicit budget distinct from the
   server's drain grace — e.g. add a new const `telemetryShutdownGrace = 5 * time.Second` and use
   `context.WithTimeout(context.Background(), telemetryShutdownGrace)` for the
   `shutdownTelemetry(shutdownCtx)` call, instead of reusing `shutdownGrace`. No test exists
   asserting shutdown timing; none required.

5. **Thread 23 (S4, auto-fix, main.go:80)** — `gin.New()` is used without
   `gin.SetMode(gin.ReleaseMode)`, so gin runs in its default debug mode (verbose stdout
   diagnostics, weaker production hardening). Fix: call `gin.SetMode(gin.ReleaseMode)` at the top
   of `buildApp` (before `gin.New()`). No test needed (this is a global mode setter with no
   externally-observable behavior change worth asserting in this test suite).

6. **Threads 4 and 12 (A4 + Q5, auto-fix — combined, main.go:83 + routes.go:11)** — `v1 :=
   app.Group("/")` mounts at the root path, not `/v1`, but the variable name and routes.go's
   comment both imply API versioning that doesn't exist; separately, routes.go's comment cites
   "applyr's AD-002 fix" — but **this project's own `AD-002`** (in `.specs/STATE.md`) is an
   unrelated decision (Toggl webhook retry semantics), so the citation collides confusingly with a
   real, differently-defined decision ID in this same repo, even though `dinherim`/`applyr` as a
   prior-project reference is itself legitimate and traceable via `design.md`. Fix: rename the
   `main.go` variable from `v1` to something accurate (e.g. `root`) since no `/v1` URL versioning
   is intended, and update `routes.go`'s `Routes` function signature/comment to match (parameter
   name and doc comment) — rewrite the comment to state the *reason* self-contained (routes must be
   registered on a group, not directly on `app`, so middleware attached to that group actually
   runs on them) without citing "AD-002" (to avoid the collision) — a brief mention that this
   mirrors a prior project's fix is fine, just without the specific, colliding ID.

7. **Thread 46 (G4, auto-fix, routes.go:13 — new file)** — `Routes()` registers POST
   `/webhooks/toggl` on the group specifically so middleware registered on that group runs, but no
   test calls `Routes()` itself (`handler_test.go`'s `newTestHandler` bypasses it via direct
   `r.POST(...)`). Add `internal/routes/routes_test.go`: build a `gin.Engine`, attach a marker
   middleware (e.g. one that sets a response header or increments a counter) to a
   `*gin.RouterGroup`, call `Routes(group, handler)` with a minimal `*webhook.Handler` (or a
   lightweight stand-in `gin.HandlerFunc` if constructing a real `*webhook.Handler` is awkward —
   prefer a real minimal `webhook.NewHandler` with a fake `sync.Processor`/`JiraClient` if
   straightforward, otherwise a package-internal test using a plain `gin.HandlerFunc` registered
   the same way), POST to `/webhooks/toggl`, and assert both that the handler ran (200-class
   response) and that the marker middleware ran (its side effect is observable).

## Cluster: internal/jira/client.go, internal/jira/client_test.go

1. **Threads 15 and 21 (P1 + S2, auto-fix — duplicate findings, same fix)** — `NewClient` falls
   back to `http.DefaultClient` (Timeout: 0) when `hc` is nil, and `di.BuildDependencies` always
   passes `nil`, so a hung/slow JIRA endpoint can block the request goroutine indefinitely. Fix: in
   `NewClient`'s nil-fallback branch, construct an explicit-timeout client instead:
   ```go
   const defaultClientTimeout = 15 * time.Second
   ...
   if hc == nil {
       hc = &http.Client{Timeout: defaultClientTimeout}
   }
   ```
   (add `"time"` import to client.go). Test: add
   `TestNewClient_NilHTTPClient_AppliesDefaultTimeout` asserting the constructed client's
   `httpClient.Timeout` field (add a small unexported accessor or check via a package-internal test
   since `httpClient` is unexported — this file is `package jira`, same-package test, so direct
   field access is fine) equals `defaultClientTimeout` and is `> 0`.

2. **Thread 29 (V5, auto-fix, client_test.go:79)** — all 3 `client_test.go` tests call `do()` with
   a nil body, so the `json.Marshal` branch and the `Content-Type: application/json` header
   (set only when `body != nil`) are never directly asserted in this file. Add
   `TestDo_NonNilBody_SetsContentTypeAndMarshalsJSON`: call `c.do(ctx, http.MethodPost, path,
   someStruct)`, assert the server observed `Content-Type: application/json` and that the request
   body, decoded, matches the marshaled input.

3. **Thread 34 (Perf P1, auto-fix, client_test.go:13)** — none of the 3 tests in this file call
   `t.Parallel()`, though each is fully self-contained (own server, own client). Add `t.Parallel()`
   as the first line of each test function (do this last, after step 2 adds its new test, so it's
   included too).

## Cluster: internal/jira/worklog.go, internal/jira/worklog_test.go

Work through in this order — step 1 restructures the file, later steps build on the result:

1. **Thread 38 (M1, auto-fix, worklog_test.go:149)** — `TestUpdateWorklog_TransientError` (149),
   `TestUpdateWorklog_PermanentError` (165), `TestDeleteWorklog_TransientError` (206),
   `TestDeleteWorklog_PermanentError` (222), `TestCreateWorklog_TransientError` (281),
   `TestCreateWorklog_PermanentError` (298) are six near-identical bodies (spin up a server
   returning a fixed status, call the method, assert `errors.As` into `*TransientError`/
   `*PermanentError`), differing only in which client method and status code. Collapse into one
   table-driven test, e.g. `TestWorklogMethods_ClassifyErrorsByStatus`, keyed by `{name string,
   call func(c *Client) error, status int, wantTransient bool}` (or two tables/two functions if
   cleaner — one for transient statuses, one for permanent), iterating with `t.Run(tc.name, ...)`.
   Preserve exactly the existing coverage (6 cases: Update×2, Delete×2, Create×2) — this step is a
   pure refactor, no new coverage yet. This also resolves thread 37 (C1)'s naming complaint (see
   below) since the collapsed table uses one consistently-named test.

2. **Thread 25 (V1, auto-fix, worklog_test.go:96)** — `FindWorklogByTogglID` only has
   `TestFindWorklogByTogglID_ServerError` (5xx); no 4xx/PermanentError case exists for it, unlike
   Create/Update/Delete. Add a `Find`-permanent-error case to the table built in step 1 (or a
   standalone `TestFindWorklogByTogglID_PermanentError` if `Find`'s different return signature
   doesn't fit the table's `call func(c *Client) error` shape cleanly — adapt the table's function
   type to accommodate both, e.g. `call func(c *Client) error` wrapping `Find`'s error-only
   extraction), asserting `errors.As(err, &permanentErr)` on a 404 response.

3. **Thread 26 (V2, auto-fix, worklog_test.go:95)** — every "transient" test in the file uses only
   5xx statuses (500/502/503); no test exercises the `429` branch of `classifyStatus`
   (`resp.StatusCode == http.StatusTooManyRequests`). Add a 429 case to the table from step 1 (on
   any one method, e.g. `CreateWorklog`), asserting it classifies as `*TransientError`.

4. **Thread 27 (V3, auto-fix, worklog_test.go:18)** — `FindWorklogByTogglID` and `CreateWorklog`
   both wrap a decode error when the 2xx response body isn't valid JSON, but no test serves a
   malformed body with a 2xx status. Add `TestFindWorklogByTogglID_MalformedJSONBody` and
   `TestCreateWorklog_MalformedJSONBody`: server returns `200`/`201` with a non-JSON body (e.g.
   `w.Write([]byte("not json"))`), assert a non-nil decode error is returned (not a panic, not a
   nil result with nil error).

5. **Thread 47 (G5, auto-fix, worklog.go:84)** — `FindWorklogByTogglID`, `UpdateWorklog`, and
   `DeleteWorklog` each wrap a `c.do()` network-level error (before any HTTP status is seen) into a
   `*TransientError`, but only `CreateWorklog` has a dedicated network-error test
   (`TestCreateWorklog_NetworkError`). Add `TestFindWorklogByTogglID_NetworkError`,
   `TestUpdateWorklog_NetworkError`, `TestDeleteWorklog_NetworkError`, mirroring
   `TestCreateWorklog_NetworkError`'s pattern (`httptest.NewServer` closed before the call),
   asserting `*TransientError`.

6. **Thread 17 (P3, auto-fix, worklog.go:82)** — `FindWorklogByTogglID`'s paginated scan has no
   `maxResults` query param, so JIRA falls back to its default page size (20), requiring many
   sequential round trips for issues with hundreds of worklogs. Fix: add
   `&maxResults=100` (or similar constant, e.g. `const findWorklogPageSize = 100`) to the URL built
   in `FindWorklogByTogglID`: `path := fmt.Sprintf("/rest/api/3/issue/%s/worklog?startAt=%d&maxResults=%d",
   url.PathEscape(issueKey), startAt, findWorklogPageSize)`. Update
   `TestFindWorklogByTogglID_Paginated` (or add an assertion) to confirm the request URL's
   `maxResults` query param is present and equals the constant.

7. **Thread 35 (Perf P2, auto-fix, worklog_test.go:18)** — none of the 14 tests in this file call
   `t.Parallel()`, though each spins its own independent `httptest.NewServer`/`Client`. Add
   `t.Parallel()` as the first line of every top-level test function in the file (including any new
   ones added above, and — inside the table-driven test from step 1 — as the first line of each
   `t.Run(tc.name, func(t *testing.T) { t.Parallel(); ... })` subtest). Do this last, after all
   other worklog_test.go changes land.

Thread 37 (C1, worklog_test.go:96/naming) needs **no separate commit** — step 1's table-driven
collapse already resolves the naming inconsistency it flagged (there's no longer a standalone
`TestFindWorklogByTogglID_ServerError` to rename; it's folded into the shared table). Document this
in its thread reply, pointing at step 1's commit.

## Cluster: internal/shared/config/config.go, internal/shared/config/config_test.go

1. **Thread 22 (S3, auto-fix, config.go:30)** — `JiraConfig.BaseURL` only has `validate:"required"`;
   nothing enforces `https://`, so a misconfigured `JIRA_BASE_URL=http://...` would transmit the API
   token as reversible Base64 Basic-Auth in cleartext. Fix: in `Load()`, after building `cfg` but
   before/alongside the existing validator call, add a manual check:
   ```go
   if cfg.Jira.BaseURL != "" && !strings.HasPrefix(cfg.Jira.BaseURL, "https://") {
       errs = append(errs, fmt.Errorf("config: Jira.BaseURL must use https, got %q", cfg.Jira.BaseURL))
   }
   ```
   placed where it can join the existing `errs` slice before the `len(errs) > 0` check (add
   `"strings"` import if not already present — it already is, check first). Test: add a
   `TestLoad` case, e.g. `{name: "JIRA_BASE_URL not https", setup: malformedEnv("JIRA_BASE_URL",
   "http://example.atlassian.net"), wantErr: true, errField: "Jira.BaseURL"}`.

2. **Thread 28 (V4, auto-fix, config_test.go:98)** — `PORT` has `validate:"required,min=1,max=65535"`,
   but only the non-numeric malformed case is tested; `PORT=0`/`PORT=70000` parse fine via
   `strconv.Atoi` and would instead exercise the validator's min/max branch, untested. Add two
   `TestLoad` cases: `{name: "PORT out of range (0)", setup: malformedEnv("PORT", "0"), wantErr:
   true, errField: "Port"}` and one for `"70000"`.

3. **Thread 42 (M5, auto-fix, config_test.go:174)** — `wantField`'s returned closure calls
   `t.Errorf` but never calls `t.Helper()`, unlike `clearConfigEnv`/`setValidRequiredEnv` in the
   same file. Add `t.Helper()` as the first line of the closure returned by `wantField`.

## Cluster: internal/toggl/parse_test.go

**Thread 33 (V9, auto-fix, parse_test.go:43)** — `descriptionPattern` caps the slug at
`[A-Z][A-Z0-9]{1,9}` (10 chars total), but the table only tests a too-short slug; neither the exact
10-char boundary (should match) nor an 11-char slug (should fail) is covered. Add two cases: a
10-char slug (e.g. `"ABCDEFGHIJ"`) that should parse with `wantOK: true`, and an 11-char slug (e.g.
`"ABCDEFGHIJK"`) that should fail with `wantOK: false`.

## Cluster: internal/jira/adf_test.go

**Thread 30 (V6, auto-fix, adf_test.go:53)** — `ExtractTogglID`'s guard is `para.Type != "paragraph"
|| len(para.Content) == 0`; the table covers the first disjunct and a differently-shaped failure,
never a paragraph node with empty/nil `Content`. Add a case to
`TestExtractTogglID_NoMarkerOrStructurallyDifferent`'s table: `{name: "paragraph with empty
content", doc: ADFDocument{Type: "doc", Version: 1, Content: []ADFNode{{Type: "paragraph", Content:
nil}}}}`.

## Cluster: internal/shared/server/server_test.go (new file)

**Thread 43 (G1, auto-fix, server.go:13)** — `Run()` has multiple untested branches: no
`server_test.go` exists at all. Add `internal/shared/server/server_test.go` covering, as many of
these as can be confidently and reliably implemented with `net`/`httptest`/table tests (implement
what's confidently doable; report any sub-case that can't be reliably implemented rather than
guessing):
1. `ListenAndServe()` returning a non-`ErrServerClosed` error is propagated by `Run()` — use an
   `*http.Server` with an already-in-use or invalid `Addr` to force an immediate error.
2. Signal-triggered graceful shutdown: start `Run()` in a goroutine against a real listener
   (`Addr: "127.0.0.1:0"` won't work directly since `http.Server.ListenAndServe` binds internally —
   use a `Handler` that responds normally, send a signal on `sigCh`, assert `Run()` returns `nil`
   promptly).
3. Forced close when Shutdown's grace period is exceeded: use a handler that blocks past a very
   short `grace` duration (e.g. `10 * time.Millisecond`), assert the "forced shutdown" error is
   returned.
4. The nil-logger fallback (`log == nil` should fall back to `slog.Default()` without panicking) —
   easiest: call `Run` with `log: nil` and a scenario that logs at least one line (e.g. the signal
   path), asserting no panic occurs.

## Cluster: internal/shared/telemetry/telemetry_test.go (new file)

**Thread 44 (G2, auto-fix, telemetry.go:38)** — only `NewMetrics` has any test coverage in this
package; `Initialize()`'s stdout-vs-OTLP branching, exporter-construction error wrapping, and the
joined-shutdown-errors func are all untested. Add `internal/shared/telemetry/telemetry_test.go`:
1. `Initialize(ctx, Config{OTLPEndpoint: ""})` wires stdout exporters and returns a working
   shutdown func (call it, assert no error).
2. A forced exporter-construction failure (e.g. `Config{OTLPEndpoint: "http://\x7f invalid"}` or
   another deliberately-invalid URL that `otlptracehttp`/`otlpmetrichttp` reject) returns a wrapped
   error, not a panic — implement this only if a reliably-invalid-URL construction failure can be
   found; otherwise report as not confidently implementable and skip it rather than guessing.

## Cluster: internal/shared/logger/logger_test.go (new file)

**Thread 45 (G3, auto-fix, logger.go:9)** — no `logger_test.go` exists: `Initialize(env)`'s
production-vs-other branching, `FromContext`'s fallback to `slog.Default()`, and the
`WithLogger`/`FromContext` round-trip are all unverified. Add `internal/shared/logger/logger_test.go`:
1. `Initialize("production")` returns a logger backed by a JSON handler (assert output is valid
   JSON when logging a line — redirect via a handler swap or check `Handler()`'s type/behavior);
   `Initialize("dev")` (or any non-"production" value) returns a text-handler-backed logger (assert
   output is NOT valid JSON).
2. `FromContext(context.Background())` (no `WithLogger` ever called) returns `slog.Default()`.
3. `FromContext(WithLogger(ctx, l))` returns exactly `l` (same pointer/identity, or same output
   behavior if pointer identity isn't practical to assert).

---

## Summary

- 47 threads fetched, 47 unresolved (0 already resolved, 0 pending review).
- 5 pushback (threads 1, 7, 13, 40, 41) — documented reasoning above, no code change.
- 42 auto-fix/apply-as-directed threads, grouped into 10 clusters (some clusters resolve multiple
  duplicate/overlapping threads with a single combined fix — noted inline above).
- 0 unclear, 0 routed-to-person.
