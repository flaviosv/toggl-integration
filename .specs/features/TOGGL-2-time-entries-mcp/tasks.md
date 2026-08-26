# Time Entries MCP Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user — do not proceed without it.**

---

**Design**: `.specs/features/TOGGL-2-time-entries-mcp/design.md`
**Status**: Draft

---

## Test Coverage Matrix

> Guidelines found: `.specs/STATE.md` AD-004 (TS package defaults: `node:test`/`node:assert/strict`, no Jest/Vitest, no live network calls) and `design.md`'s Tech Decisions table (same, plus "faked via `node:http`"). `mcp/` is this repo's first TypeScript package — no existing TS test files to sample. `docs/codebase/TESTING.md` (to-jira, Go) supplies the cross-language floor this repo already holds itself to: co-located tests, all-branches coverage for business logic, build-gate-only for thin wiring, hard "no live network calls" constraint. Confirm before Execute.

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| --- | --- | --- | --- | --- |
| `src/matching/match-project.ts` (pure domain logic) | unit | All branches; 1:1 to TEM-13..TEM-17 + every listed Edge Case (empty bracket remainder, no-trailing-text code, non-anchored bracket, lowercase tag) | `src/matching/*.test.ts` | `npm test` |
| `src/logger.ts` (stderr-only contract) | unit | Asserts every log call reaches `console.error` and never `console.log`/stdout (TEM-24 AC4) | `src/logger.test.ts` | `npm test` |
| `src/config.ts` (env validation) | unit | All branches: missing token, missing/non-numeric/non-positive workspace id, both missing (joined message), default cache path, explicit `TOGGL_CACHE_PATH` (TEM-01) | `src/config.test.ts` | `npm test` |
| `src/toggl/client.ts` (Toggl API boundary) | unit (`node:http` fake server) | All 6 methods (list/get/create/update/delete time entries, list projects) + Basic Auth header shape + error mapping: 2xx success, 404, 429 with/without `Retry-After`, other non-2xx, network/timeout failure (TEM-02 AC2, TEM-24 AC1-AC3) | `src/toggl/client.test.ts` | `npm test` |
| `src/errors.ts` (error → tool-result shaping) | unit | All branches: `TogglApiError` with/without `retryAfter`, `notFound` flag on 404, `TogglNetworkError`, matching `no_match`, matching `ambiguous` | `src/errors.test.ts` | `npm test` |
| `src/cache/project-cache.ts` (TTL cache orchestration) | unit (`node:http` fake + temp cache dir) | All branches: no cache (fetch+write), fresh cache (zero calls), stale cache (refetch+overwrite), `forceRefresh`, corrupt/missing/unreadable file (treated as miss), unwritable cache dir (serves from memory), refetch failure with a stale cache available (`stale_cache` warning), refetch failure with no cache at all (rethrow) (TEM-08..TEM-12) | `src/cache/project-cache.test.ts` | `npm test` |
| `src/time-entries/curate.ts` (pure mapping) | unit | Entry with a resolved project name, entry with `project_id` null/absent → `project: null` (TEM-05 AC3) | `src/time-entries/curate.test.ts` | `npm test` |
| `src/tools/*.ts` (6 tool handlers) + `src/tools/schemas.ts` | integration (MCP SDK `Client`↔`McpServer` over `InMemoryTransport` + `node:http` fake Toggl) | Every story's Independent Test + every AC: happy path, schema-validation rejection (zero Toggl calls), matching no-match/ambiguous, not-found, stale-cache warning passthrough, workspace-filtering, exact request-count assertions (TEM-03..TEM-07, TEM-15..TEM-23) | `src/tools/*.test.ts` | `npm test` |
| `src/index.ts` (bootstrap) | integration (spawned child process) | Missing/invalid env → non-zero exit naming the variable, zero MCP handshake bytes on stdout; valid env → successful handshake + tool list (TEM-01 AC1, TEM-24 AC4) | `src/index.test.ts` | `npm test` |
| `src/toggl/generated.ts`, `mcp/openapi/*`, `package.json`, `tsconfig.json`, `.env.sample`, `.gitignore` | none | Thin/generated/config artifacts, no branching logic — build gate only | — | `npm run build` |

## Gate Check Commands

> Package doesn't exist yet — these commands are established by T1 and used by every later task. `npm test` runs `npm run build` (tsc, strict) followed by `node --test` over the compiled output, so it also serves as the type-check gate. There is no separate integration/e2e command: the one Node test runner covers unit and integration layers alike (fakes stand in for Toggl in both), so Quick and Full are the same command for this package.

| Gate Level | When to Use | Command |
| --- | --- | --- |
| Quick | After a task touching only pure/unit-tested code | `npm test` |
| Full | After a task touching a tool handler or the bootstrap (integration-tested code) | `npm test` |
| Build | After phase completion or a config/generated-artifact-only task | `npm run build` |

---

## Execution Plan

Phases are ordered and run sequentially — each phase completes before the next begins, and tasks within a phase execute in order.

### Phase 1: Foundation

```
T1 → T2 → T3 → T4
```

### Phase 2: Toggl Integration Layer

```
T5 → T6 → T7 → T8 → T9
```

### Phase 3: Tools & Bootstrap

```
T10 → T11 → T12 → T13 → T14 → T15
```

---

## Task Breakdown

### T1: Scaffold the `mcp/` package

**What**: `mcp/package.json` (name, `"type": "module"`, deps `@modelcontextprotocol/sdk`, `zod`, `dotenv`; devDeps `typescript`, `@types/node`, `openapi-typescript`, `swagger2openapi`; scripts `build` = `tsc -p tsconfig.json`, `test` = `npm run build && node --test dist`, `generate:openapi` = swagger2openapi → openapi-typescript pipeline), `mcp/tsconfig.json` (strict, `module`/`moduleResolution: NodeNext`, `outDir: dist`), `mcp/.gitignore` (`.env`, `dist/`, `node_modules/`), `mcp/.env.sample` (`TOGGL_API_TOKEN`, `TOGGL_WORKSPACE_ID`, `TOGGL_CACHE_PATH` — all with placeholder values and a one-line comment each; matches design.md's `Config` interface).
**Where**: `mcp/package.json`, `mcp/tsconfig.json`, `mcp/.gitignore`, `mcp/.env.sample`
**Depends on**: None
**Reuses**: `to-jira/.env.sample`'s required/optional comment-block convention; AD-004 tooling defaults
**Requirement**: TEM-02 AC4 (`.env.sample` committed, `.env` gitignored)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `npm install` succeeds inside `mcp/`
- [ ] `npm run build` succeeds on the empty `src/` (add a placeholder `src/index.ts` exporting nothing, deleted/replaced by T15)
- [ ] `.env.sample` lists exactly `TOGGL_API_TOKEN`, `TOGGL_WORKSPACE_ID`, `TOGGL_CACHE_PATH` with placeholders
- [ ] `.gitignore` excludes `.env`, `dist/`, `node_modules/`

**Tests**: none
**Gate**: build

**Commit**: `chore(mcp): scaffold TypeScript package`

---

### T2: Implement ticket-code matching

**What**: `extractTicketCode(description)`, `resolveProject(code, allProjects, workspaceId)`, the `MatchResult` union type — exact algorithm from design.md's Matching Algorithm section.
**Where**: `mcp/src/matching/match-project.ts` (+ `match-project.test.ts`)
**Depends on**: T1
**Reuses**: `to-jira/internal/toggl/parse.go`'s regex convention (TEM-13)
**Requirement**: TEM-13, TEM-14, TEM-15, TEM-16 (type only — bypass logic lives in the tool tasks), TEM-17

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `extractTicketCode` returns the code for `[JSP-10] Creating form`, returns the code (empty trailing group) for `[JSP-10]`, returns `null` for `Fixing [JSP-10] today` and for `[jsp-10] x`
- [ ] `resolveProject` strips a leading `[...]` prefix and matches case-insensitively; a project named exactly `[teachmeto.ai]` never matches any code
- [ ] `resolveProject` filters to `active && workspaceId === target` before matching, returns `"matched"` on exactly one hit, `"ambiguous"` listing all matches on >1, `"no_match"` listing all active candidates on 0
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥10 (covers every Edge Cases bullet plus TEM-14/15/16 branches)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(mcp): add ticket-code to project matching`

---

### T3: Implement the stderr-only logger

**What**: `log(level: "info" | "warn" | "error", message: string, meta?: Record<string, unknown>)` writing to `console.error` only.
**Where**: `mcp/src/logger.ts` (+ `logger.test.ts`)
**Depends on**: T1
**Reuses**: n/a (new)
**Requirement**: TEM-24 AC4

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Calling `log` for each level writes exactly one JSON-ish line via `console.error`
- [ ] `console.log` is never called (spied and asserted zero calls)
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥3

**Tests**: unit
**Gate**: quick

**Commit**: `feat(mcp): add stderr-only logger`

---

### T4: Implement config loading and validation

**What**: `loadConfig(env: NodeJS.ProcessEnv): Config` — loads `.env` via `dotenv`, validates `TOGGL_API_TOKEN` (required, non-empty) and `TOGGL_WORKSPACE_ID` (required, positive integer), defaults `TOGGL_CACHE_PATH` to `~/.cache/toggl-mcp/projects.json`, throws one `ConfigError` joining every missing/invalid variable.
**Where**: `mcp/src/config.ts` (+ `config.test.ts`)
**Depends on**: T1
**Reuses**: `to-jira/internal/shared/config`'s joined-error pattern (TEM-01)
**Requirement**: TEM-01

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Missing `TOGGL_API_TOKEN` throws `ConfigError` naming it
- [ ] Missing, non-numeric, zero, or negative `TOGGL_WORKSPACE_ID` throws `ConfigError` naming it
- [ ] Both invalid at once → one `ConfigError` naming both (joined, not first-error-only)
- [ ] Unset `TOGGL_CACHE_PATH` defaults to `~/.cache/toggl-mcp/projects.json`; set value is used verbatim
- [ ] Valid env produces a `Config` with no error thrown
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥6

**Tests**: unit
**Gate**: quick

**Commit**: `feat(mcp): add config loading and validation`

---

### T5: Commit Toggl OpenAPI spec and generate types

**What**: Locate/obtain Toggl's Swagger 2.0 spec (public source; if none is found, hand-author a minimal Swagger 2.0 doc covering only the 6 endpoints this client calls — flag which path was taken), commit it at `mcp/openapi/toggl.swagger2.json`, convert it once via `swagger2openapi` to `mcp/openapi/toggl.openapi3.json` (both committed), then generate `mcp/src/toggl/generated.ts` via `openapi-typescript`. Wire the `generate:openapi` npm script (from T1) to reproduce this.
**Where**: `mcp/openapi/toggl.swagger2.json`, `mcp/openapi/toggl.openapi3.json`, `mcp/src/toggl/generated.ts`
**Depends on**: T1
**Reuses**: n/a (first codegen in this repo)
**Requirement**: supports TEM-03, TEM-07, TEM-08, TEM-19, TEM-21, TEM-23 (typed request/response shapes)

**Tools**:

- MCP: `context7` (confirm current `openapi-typescript`/`swagger2openapi` CLI usage)
- Skill: NONE

**Done when**:

- [ ] `mcp/openapi/toggl.swagger2.json` and `toggl.openapi3.json` are both committed
- [ ] `generated.ts` includes types covering time entries and projects request/response bodies
- [ ] `npm run generate:openapi` reproduces `generated.ts` byte-identically from the committed spec
- [ ] `npm run build` typechecks against the generated types

**Tests**: none
**Gate**: build

**Commit**: `chore(mcp): commit Toggl OpenAPI spec and generate types`

---

### T6: Implement the Toggl API client

**What**: `class TogglClient` — constructor `{ apiToken, baseUrl? }`; `listTimeEntries`, `getTimeEntry`, `createTimeEntry`, `updateTimeEntry`, `deleteTimeEntry`, `listProjects`; `TogglApiError`/`TogglNetworkError` classes; Basic Auth header (`base64(apiToken + ":api_token")`); no retry/backoff/throttle.
**Where**: `mcp/src/toggl/client.ts` (+ `client.test.ts`)
**Depends on**: T1, T5
**Reuses**: `jira.TransientError`/`PermanentError`-style typed-error shape from `to-jira/internal/jira`
**Requirement**: TEM-02 AC2, TEM-03, TEM-07 AC6/AC7, TEM-08 AC1, TEM-19 AC3, TEM-21 AC2, TEM-23 AC1, TEM-24 AC1-AC3

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Every method issues exactly the documented method+path against a `node:http` fake server
- [ ] Every request carries `Authorization: Basic <base64(token:api_token)>`; the header is asserted, never logged
- [ ] 2xx responses resolve with the parsed body for every method
- [ ] Non-2xx (404, other) throws `TogglApiError { status, method, path, body }`
- [ ] `429` with a `Retry-After` header throws `TogglApiError` carrying `retryAfter`; without the header, `retryAfter` is absent
- [ ] A network failure (server closes the socket) throws `TogglNetworkError { operation, cause }`
- [ ] No method retries, sleeps, or issues a second request on any failure
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥14

**Tests**: unit
**Gate**: quick

**Commit**: `feat(mcp): add Toggl API client`

---

### T7: Implement error-to-tool-result shaping

**What**: `toErrorResult(err: TogglApiError | TogglNetworkError | MatchingError): CallToolResult` — maps each error kind to the shapes in design.md's Error Handling Strategy table (`toggl_api` with `retryAfter?`/`notFound?`, `network`, `matching` with `extractedCode`/`candidates`).
**Where**: `mcp/src/errors.ts` (+ `errors.test.ts`)
**Depends on**: T2, T6
**Reuses**: n/a (new)
**Requirement**: TEM-15, TEM-24 AC1, TEM-24 AC2, TEM-24 AC3

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `TogglApiError` with `status: 404` maps to `{ error: { type: "toggl_api", status: 404, notFound: true, ... } }`
- [ ] `TogglApiError` with `retryAfter` set carries it through unchanged; without it, the field is absent
- [ ] `TogglNetworkError` maps to `{ error: { type: "network", message, operation } }`
- [ ] A `"no_match"`/`"ambiguous"` `MatchResult` maps to `{ error: { type: "matching", extractedCode, candidates } }`
- [ ] Every mapped result sets `isError: true`
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥6

**Tests**: unit
**Gate**: quick

**Commit**: `feat(mcp): add error-to-tool-result mapping`

---

### T8: Implement the project cache

**What**: `getProjects(client, cachePath, opts?: { forceRefresh?: boolean }): Promise<{ projects: CachedProject[]; warning?: StaleCacheWarning }>` — the full read/freshness/refetch/atomic-write/degrade strategy from design.md's Cache Strategy section.
**Where**: `mcp/src/cache/project-cache.ts` (+ `project-cache.test.ts`)
**Depends on**: T3, T6
**Reuses**: n/a (new)
**Requirement**: TEM-08, TEM-09, TEM-10, TEM-11, TEM-12

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] No cache file → one `listProjects` call, cache written with `fetchedAt` + `projects`
- [ ] Fresh cache (<7 days) → zero `listProjects` calls
- [ ] Stale cache (≥7 days, back-dated `fetchedAt`) → one refetch, cache overwritten
- [ ] `forceRefresh: true` → always refetches regardless of freshness, returns `{ count, fetchedAt }`-shaped data for the `refresh_projects` tool to use
- [ ] Cache file missing, malformed JSON, or missing `fetchedAt`/`projects` → treated as a miss, refetches, never throws the parse error
- [ ] Cache directory unwritable → write failure logged via T3's logger, freshly-fetched list still returned, call succeeds
- [ ] Stale cache + refetch fails (network/429/5xx) + a stale cache exists → returns the stale `projects` plus a `stale_cache` warning with age and underlying error
- [ ] Stale/absent cache + refetch fails + no cache at all → rethrows the underlying `TogglApiError`/`TogglNetworkError`
- [ ] Concurrent-write edge case: cache write goes through a temp-file-then-rename, verified by asserting no `.tmp-*` file is left behind after a successful write
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥10

**Tests**: unit
**Gate**: quick

**Commit**: `feat(mcp): add TTL project cache`

---

### T9: Implement curated time-entry mapping

**What**: `toCuratedEntry(entry: RawTimeEntry, projectsById: Map<number, string>): CuratedTimeEntry` — five-field shape (`id`, `description`, `start`, `stop`, `project`); `project` is `null` when the entry has no `project_id`, otherwise `projectsById.get(project_id) ?? null`.
**Where**: `mcp/src/time-entries/curate.ts` (+ `curate.test.ts`)
**Depends on**: T5 (generated `RawTimeEntry` type)
**Reuses**: n/a (new)
**Requirement**: TEM-05

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Entry with a `project_id` present in `projectsById` → `project` is the resolved name
- [ ] Entry with `project_id` null/absent → `project: null`
- [ ] Entry with a `project_id` not present in `projectsById` (stale/mismatched cache) → `project: null`, not a thrown error
- [ ] Output carries exactly the five fields — no extras leak through from the raw entry
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥4

**Tests**: unit
**Gate**: quick

**Commit**: `feat(mcp): add curated time-entry mapping`

---

### T10: Implement `list_time_entries` and `get_time_entry` tools

**What**: `src/tools/schemas.ts` (`rfc3339Timestamp`, `dateOrTimestamp`, `positiveId` zod validators shared by every tool); `registerListTimeEntries`/`registerGetTimeEntry`, each `register<Tool>(server, deps): void`. Also establishes the shared test harness (SDK `Client`↔`McpServer` over `InMemoryTransport`, `node:http` fake Toggl) reused by every later tool task.
**Where**: `mcp/src/tools/schemas.ts`, `mcp/src/tools/list-time-entries.ts`, `mcp/src/tools/get-time-entry.ts` (+ matching `.test.ts` files)
**Depends on**: T6, T7, T8, T9
**Reuses**: n/a (first tool registrations)
**Requirement**: TEM-03, TEM-04, TEM-05, TEM-06, TEM-07

**Tools**:

- MCP: `context7` (confirm `@modelcontextprotocol/sdk` v1.x `InMemoryTransport`/`Client` test-harness API, since design.md only confirmed the server-side `registerTool` surface)
- Skill: NONE

**Done when**:

- [ ] `list_time_entries` issues exactly one Toggl request per call, regardless of range width
- [ ] Invalid `start_date`/`end_date` (bad format, `end_date < start_date`) is rejected by the SDK's schema validation before any Toggl request — asserted via zero outbound requests
- [ ] Result entries carry exactly the five curated fields; entries from a foreign workspace are omitted
- [ ] An empty range returns a successful empty list, not an error
- [ ] `get_time_entry` issues exactly one Toggl request and returns the curated shape; a 404 from Toggl returns a structured not-found error via T7, not an empty success
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥10

**Tests**: integration
**Gate**: full

**Commit**: `feat(mcp): add list_time_entries and get_time_entry tools`

---

### T11: Implement `create_time_entry` tool

**What**: `registerCreateTimeEntry` — validates `description`/`start`/`stop` (RFC3339, `stop` strictly after `start`), resolves the project per the Matching Algorithm orchestration (T2 + T8), computes `duration`, issues one `POST`, returns the curated result.
**Where**: `mcp/src/tools/create-time-entry.ts` (+ `create-time-entry.test.ts`)
**Depends on**: T2, T6, T7, T8, T9, T10
**Reuses**: T10's zod schemas and SDK test harness
**Requirement**: TEM-16, TEM-17, TEM-18, TEM-19, TEM-20

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Invalid `start`/`stop` (bad RFC3339, `stop` not after `start`) is rejected before any Toggl request
- [ ] `[JSP-10] Creating form` for a 2-hour window → outbound `POST` body carries the matched project id and `duration: 7200`
- [ ] Ambiguous/no-match code → the matching error is returned and zero `POST` requests are issued
- [ ] Explicit `project_id` → matching is skipped entirely and `listProjects`/the cache is never read
- [ ] Untagged description with no `project_id` → `POST` omits `project_id`, result reports `project: null`
- [ ] Omitted `workspace_id` uses `TOGGL_WORKSPACE_ID`; a supplied value overrides it for that call only
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥10

**Tests**: integration
**Gate**: full

**Commit**: `feat(mcp): add create_time_entry tool`

---

### T12: Implement `update_time_entry` tool

**What**: `registerUpdateTimeEntry` — requires `id` + at least one of `description`/`start`/`stop`/`project_id`, fetches the current entry, merges supplied fields, re-runs matching only when `description` is supplied without an explicit `project_id`, validates the merged `start`/`stop`, issues one `PUT`.
**Where**: `mcp/src/tools/update-time-entry.ts` (+ `update-time-entry.test.ts`)
**Depends on**: T2, T6, T7, T8, T9, T10
**Reuses**: T10's zod schemas and SDK test harness

**Requirement**: TEM-21, TEM-22

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] A call supplying only `id` is rejected before any Toggl request
- [ ] Updating only `description` from `[JSP-10] a` to `[OIQ-3] b` → outbound `PUT` carries the OIQ project id and the original `start`/`stop` unchanged; exactly two requests (`GET` then `PUT`)
- [ ] Updating `start`/`stop` without `description` → the entry's existing `project_id` passes through untouched and `listProjects`/the cache is never read
- [ ] A merged `stop` not strictly after the merged `start` is rejected and no `PUT` is issued
- [ ] Successful update returns the curated five-field shape
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥8

**Tests**: integration
**Gate**: full

**Commit**: `feat(mcp): add update_time_entry tool`

---

### T13: Implement `delete_time_entry` tool

**What**: `registerDeleteTimeEntry` — issues one `DELETE`, returns a success result naming the deleted id; a Toggl 404 becomes a structured not-found error.
**Where**: `mcp/src/tools/delete-time-entry.ts` (+ `delete-time-entry.test.ts`)
**Depends on**: T6, T7, T10
**Reuses**: T10's zod schemas and SDK test harness
**Requirement**: TEM-23

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Deleting an existing id issues exactly one `DELETE` and returns a success result naming that id
- [ ] Deleting an id the fake server 404s returns a structured not-found error, not a success
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥4

**Tests**: integration
**Gate**: full

**Commit**: `feat(mcp): add delete_time_entry tool`

---

### T14: Implement `refresh_projects` tool

**What**: `registerRefreshProjects` — calls `getProjects` with `forceRefresh: true`, returns `{ count, fetchedAt }` on success; follows the same stale/hard-error branches as any other caller on failure.
**Where**: `mcp/src/tools/refresh-projects.ts` (+ `refresh-projects.test.ts`)
**Depends on**: T8, T10
**Reuses**: T10's zod schemas and SDK test harness
**Requirement**: TEM-09 AC4

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Calling with a fresh cache still forces a refetch and overwrites the cache
- [ ] Success result reports the refetched project count and the new `fetchedAt`
- [ ] A refetch failure with no prior cache surfaces the underlying Toggl error via T7
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥4

**Tests**: integration
**Gate**: full

**Commit**: `feat(mcp): add refresh_projects tool`

---

### T15: Wire the bootstrap entrypoint

**What**: `src/index.ts` — `loadConfig`, construct `TogglClient` + cache path, call all 6 `register<Tool>` functions once, connect `StdioServerTransport`; on a `ConfigError`, log every missing/invalid var to stderr and `process.exit(1)` before registering any tool or connecting the transport.
**Where**: `mcp/src/index.ts` (+ `index.test.ts`)
**Depends on**: T4, T10, T11, T12, T13, T14
**Reuses**: n/a (bootstrap is new)
**Requirement**: TEM-01 AC1, TEM-24 AC4

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Spawned with `TOGGL_API_TOKEN` unset → non-zero exit, stderr names the missing variable, zero bytes written to stdout (no MCP handshake attempted)
- [ ] Spawned with valid env → successful MCP handshake and a tool list of exactly 6 tools
- [ ] Every tool from T10-T14 is registered exactly once
- [ ] Gate passes: `npm test`
- [ ] Test count: ≥3

**Tests**: integration
**Gate**: full

**Commit**: `feat(mcp): wire bootstrap entrypoint`

---

## Phase Execution Map

Visual representation of task ordering. Phases run in sequence, and tasks within a phase run in order:

```
Phase 1 → Phase 2 → Phase 3

Phase 1:  T1 ──→ T2 ──→ T3 ──→ T4
Phase 2:  T5 ──→ T6 ──→ T7 ──→ T8 ──→ T9
Phase 3:  T10 ──→ T11 ──→ T12 ──→ T13 ──→ T14 ──→ T15
```

Execution is strictly sequential — there is no intra-phase parallelism. A single agent (or batch worker) works one task at a time, in order.

**How phase-based execution works**: at Execute, the agent counts total tasks (15) and packs whole phases into ~7-task batches (Phase 1 = 4, Phase 2 = 5, Phase 3 = 6 → 3 batches, close to the budget already). Since this exceeds ~8 tasks total, the agent offers sub-agent batch dispatch before starting — see the skill's Sub-Agent Delegation section.

---

## Task Granularity Check

| Task | Scope | Status |
| --- | --- | --- |
| T1: Scaffold package | 1 package config surface (package.json + tsconfig + gitignore + env sample, one cohesive scaffold) | ✅ Granular |
| T2: Matching | 1 module (2 pure functions + 1 type) | ✅ Granular |
| T3: Logger | 1 function | ✅ Granular |
| T4: Config | 1 function | ✅ Granular |
| T5: OpenAPI codegen | 1 pipeline, 3 generated/committed artifacts | ✅ Granular |
| T6: Toggl client | 1 class, 6 methods sharing one auth/error-mapping mechanism | ⚠️ OK — cohesive (one interface, one shared behavior, matches design.md's single-file component) |
| T7: Error mapping | 1 function | ✅ Granular |
| T8: Project cache | 1 function (one interface, one strategy) | ✅ Granular |
| T9: Curate mapping | 1 function | ✅ Granular |
| T10: list/get tools | 2 tool registrations + shared schemas + shared test harness (first-of-kind setup cost) | ⚠️ OK — cohesive (both are simple reads sharing the same schema/harness bootstrap) |
| T11: create tool | 1 tool registration | ✅ Granular |
| T12: update tool | 1 tool registration | ✅ Granular |
| T13: delete tool | 1 tool registration | ✅ Granular |
| T14: refresh_projects tool | 1 tool registration | ✅ Granular |
| T15: Bootstrap | 1 file (entrypoint wiring) | ✅ Granular |

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| --- | --- | --- | --- |
| T1 | None | (phase start) | ✅ Match |
| T2 | T1 | T1→T2 | ✅ Match |
| T3 | T1 | T2→T3 (T1 precedes T3 in sequence) | ✅ Match |
| T4 | T1 | T3→T4 (T1 precedes T4 in sequence) | ✅ Match |
| T5 | T1 | (phase start) — T1 completed in prior phase | ✅ Match |
| T6 | T1, T5 | T5→T6 | ✅ Match |
| T7 | T2, T6 | T6→T7 (T2 completed in prior phase) | ✅ Match |
| T8 | T3, T6 | T7→T8 (T3, T6 both precede T8 in sequence) | ✅ Match |
| T9 | T5 | T8→T9 (T5 precedes T9 in sequence) | ✅ Match |
| T10 | T6, T7, T8, T9 | (phase start) — all completed in prior phase | ✅ Match |
| T11 | T2, T6, T7, T8, T9, T10 | T10→T11 (T2 completed in prior phase) | ✅ Match |
| T12 | T2, T6, T7, T8, T9, T10 | T11→T12 (all deps precede T12 in sequence) | ✅ Match |
| T13 | T6, T7, T10 | T12→T13 (all deps precede T13 in sequence) | ✅ Match |
| T14 | T8, T10 | T13→T14 (all deps precede T14 in sequence) | ✅ Match |
| T15 | T4, T10, T11, T12, T13, T14 | T14→T15 (T4 completed in prior phase, rest precede in sequence) | ✅ Match |

**Rules confirmed**: no task depends on a later-numbered task; every dependency is satisfied by the time its dependent task runs, whether via a direct diagram arrow or by completing in an earlier phase.

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| --- | --- | --- | --- | --- |
| T1: Scaffold package | package.json/tsconfig/.env.sample/.gitignore | none | none | ✅ OK |
| T2: Matching | `src/matching/match-project.ts` | unit | unit | ✅ OK |
| T3: Logger | `src/logger.ts` | unit | unit | ✅ OK |
| T4: Config | `src/config.ts` | unit | unit | ✅ OK |
| T5: OpenAPI codegen | `src/toggl/generated.ts`, `mcp/openapi/*` | none | none | ✅ OK |
| T6: Toggl client | `src/toggl/client.ts` | unit | unit | ✅ OK |
| T7: Error mapping | `src/errors.ts` | unit | unit | ✅ OK |
| T8: Project cache | `src/cache/project-cache.ts` | unit | unit | ✅ OK |
| T9: Curate mapping | `src/time-entries/curate.ts` | unit | unit | ✅ OK |
| T10: list/get tools | `src/tools/schemas.ts`, `list-time-entries.ts`, `get-time-entry.ts` | integration | integration | ✅ OK |
| T11: create tool | `src/tools/create-time-entry.ts` | integration | integration | ✅ OK |
| T12: update tool | `src/tools/update-time-entry.ts` | integration | integration | ✅ OK |
| T13: delete tool | `src/tools/delete-time-entry.ts` | integration | integration | ✅ OK |
| T14: refresh_projects tool | `src/tools/refresh-projects.ts` | integration | integration | ✅ OK |
| T15: Bootstrap | `src/index.ts` | integration | integration | ✅ OK |

No violations. No task defers its tests to a later task; every task that creates a code layer with a required test type includes those tests in the same task.

---

## Tips

- **Phases are ordered** — Each phase completes before the next; tasks run in order within a phase
- **Reuses = Token saver** — Always reference existing code
- **Tools per task** — MCPs and Skills prevent wrong approaches
- **Dependencies are gates** — Clear what blocks what
- **Done when = Testable** — If you can't verify it, rewrite it
- **Requirement ID = Traceable** — Every task traces back to a spec requirement
- **One commit per task** — Plan the commit message format in advance (Conventional Commits, no co-authoring trailers, per project convention)

---

## Task Verification Standards

Every task MUST follow the `Done when` + `Tests` + `Gate` fields defined in the **Task Breakdown** above. Each `Done when` entry is specific, testable (binary pass/fail), and references the gate check command from **Gate Check Commands**. Expected test counts are floors to prevent silent deletions, not caps.
