# Time Entries MCP Specification

**Feature:** TOGGL-2 · **Scope size:** Large (multi-component: new TS package, MCP tool surface, Toggl API client, project cache, matching engine)
**Source of scope:** `.specs/features/TOGGL-2-time-entries-mcp/grilling-session.md` (3 rounds, 15 questions, user-confirmed)

## Problem Statement

Reading and correcting Toggl time entries today means opening the Toggl web UI by hand — there is no way for an agent to answer "what did I track last week?" or to fix a mistyped entry without a human clicking through. `to-jira` already syncs Toggl → JIRA one-way via webhooks, but nothing exposes Toggl's own Time Entries API to an agent. TOGGL-2 adds a stateless TypeScript MCP server (stdio) that gives Claude direct read/create/update/delete access to Toggl time entries, and removes the most error-prone part of creating one by hand: picking the right project, which is auto-resolved from the `[SLUG-NUMBER]` ticket tag this monorepo already uses.

## Goals

- [ ] An agent can list every time entry in an arbitrary date range in **one** Toggl API call, receiving a curated shape (id, description, start, stop, project).
- [ ] An agent can create, update, and delete time entries without the user opening Toggl.
- [ ] A time entry described as `[JSP-10] Creating form` is attached to the correct Toggl project automatically, or fails loudly with the candidate list — never silently attached to the wrong one.
- [ ] The server stays stateless in the `to-jira` sense (AD-001): no database, no networked store. The only persisted artifact is a local JSON cache of Projects, refreshable on demand.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| --- | --- |
| Bulk delete (`bulk_delete_time_entries`) | Toggl exposes **no true bulk-delete endpoint** — `/workspaces/{wid}/time_entries/{ids}` offers `patch` only, and that PATCH is a JSON-Patch on *fields* (`op`/`path`/`value`) that cannot delete an entry. A bulk tool would therefore issue N sequential single `DELETE` calls, costing exactly the same N of the 30 requests/hour as looping `delete_time_entry`. It saves no requests, so it is cut for v1 — Claude can loop `delete_time_entry` itself when it needs to remove several entries. |
| Bulk update (`PATCH .../time_entries/{ids}`) | Q4 — user chose not to expose the bulk field-edit endpoint, despite it existing. |
| CI pipeline | Consistent with `to-jira`'s explicit deferral (`.specs/features/to-jira/spec.md` Out of Scope). |
| Client-side rate throttling, retry, or backoff | Q7 — calls fire freely; Toggl's `429` is surfaced verbatim to the agent, which decides what to do. |
| Creating / updating / deleting Toggl projects or clients | Projects are read-only here, needed solely for ticket-code matching (Q9). |
| HTTP / SSE remote transport | Q1 — stdio only, single local operator. |
| `list_projects` tool | Matching errors already return the candidate project list (id + name), which is the only confirmed need for project discovery. Trivial to add later if a real use case appears. |
| Multi-workspace / multi-account support | Q3 — one default workspace via `TOGGL_WORKSPACE_ID`, with an optional per-call override. |
| npm workspaces / monorepo build tooling | Q2 — flat sibling directory `mcp/`, self-contained `package.json`. |
| OpenTelemetry instrumentation | AD-003 scopes the OTel pattern to long-lived *services*. This is a short-lived stdio child process with no collector to export to; stderr logging is the whole observability story (see TEM-24). |
| Redis or any shared / networked cache | Q10 — contradicts the project's own statelessness requirement and AD-001, and adds a failure mode (k3d Redis down ⇒ matching dead) a local file cache does not. |
| Running / live-timer support (start, stop, `current`) | Seed constraint — every operation specifies explicit start and stop times. |
| Tags, billable flags, task assignment on entries | Not requested; the curated read shape (Q5) deliberately excludes them. |
| Time-entry response caching | Q8 — reads always hit Toggl, so the agent never reasons about stale time data. |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here — nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --- | --- | --- | --- |
| **Toggl has no bulk-delete endpoint** | Bulk delete is **dropped from v1** — only `delete_time_entry` ships | Verified against Toggl's Swagger 2.0 spec: `/workspaces/{wid}/time_entries/{ids}` exposes `patch` only; `delete` exists solely on the single-entry path. A bulk tool would cost the same N of the 30 requests/hour as looping single deletes, so it earns nothing. See Out of Scope. | **y — user confirmed the cut** |
| Clients endpoint (`GET /me/clients`) is not called at all | Dropped from scope | Q12 confirmed the `[client]` prefix is literally inside the project's own `name`, and the spec shows `models.Project` already carries `client_name`/`client_id`. Fetching Clients separately would burn an API call for data the matcher never reads. Grilling said "+ Clients **if needed**" — it is not. | n |
| Project override parameter shape | `project_id` (number) only — no `project_name` override | Match errors return candidate `id` + `name`, so the agent always has an id to hand back. A second name-based resolution path would reintroduce the exact ambiguity the error exists to prevent. | **n — narrows grilling's "`project_id`/`project_name`"** |
| Description with no `[CODE-N]` tag and no `project_id` override | Entry is created/updated **with no project** (`project_id` omitted), and the result explicitly reports `project: null` | Toggl permits project-less entries. Erroring would block a legitimate untagged entry; "never silently guess" governs *ambiguity*, not *absence*, and the null in the result makes the outcome visible rather than silent. | n |
| Matching comparison precision | Strip a leading `[...]` + following whitespace from the project's `name`, then compare the remainder to the extracted code with **exact, case-insensitive equality** — never substring or fuzzy | `[teachmeto.ai] JSP` → `JSP` matches code `JSP`; `Staff Engineer Test` (no bracket) → whole name, no match. Substring matching would make `JSP` match a hypothetical `JSPX`, which is precisely the silent-wrong-project failure Q3 forbids. | y (mechanism), n (exact-equality precision) |
| Archived / inactive projects as match candidates | Excluded — the cache is populated with `include_archived=false` and only `active: true` projects are candidates | Matching a dead project produces an entry the user cannot see in normal Toggl views. | n |
| Project cache location | `~/.cache/toggl-mcp/projects.json`, overridable via `TOGGL_CACHE_PATH` | An MCP server is spawned by the client with an unpredictable working directory, so a repo-relative path is unreliable. Keeps generated state out of the git tree. | n |
| Project cache TTL | Fixed **7 days**, not configurable by env var | Q10 said "~1 week". `refresh_projects` (TEM-09) is the escape hatch, so a TTL env var would be unused config surface. | y (duration), n (non-configurable) |
| Stale cache + Toggl unreachable | Serve the **stale** cache and attach a `stale_cache` warning to the result, rather than failing the call | With a 30 req/hour ceiling, a `429` during a refresh must not take project matching offline for the rest of the hour. Degrading loudly beats failing hard. | n |
| `update_time_entry` write semantics | **Read-modify-write**: `GET /me/time_entries/{id}`, merge the caller's fields, then `PUT` the full entry — 2 API calls per update | Toggl's spec does not document whether `PUT` treats omitted fields as "unchanged" or "clear". Guessing risks silently wiping `start`/`stop`. Flagged as uncertain rather than fabricated (Knowledge Verification Chain Step 5). | n |
| Read scoping to the configured workspace | `list_time_entries` / `get_time_entry` read via `/me/time_entries*` (no workspace path param) and then **filter client-side** to `TOGGL_WORKSPACE_ID` (or the per-call override) | `/me/*` returns entries across every workspace the token can see; without the filter, reads and writes would disagree about what "the workspace" means. | n |
| Generated types are committed | The `swagger2openapi`-converted spec **and** the `openapi-typescript` output are both committed to the repo | Q15 chose types-only codegen. Committing both makes builds reproducible offline and makes the diff of a Toggl schema change reviewable. | n |
| `created_with` on create | Set to a fixed identifier for this server | The Toggl spec states it "must be provided when creating a time entry". | n |

**Open questions:** none — all resolved or logged above. The rows marked **n** are agent-chosen defaults awaiting user confirmation.

---

## User Stories

### P1: Configuration and authentication ⭐ MVP

**User Story**: As the operator, I want the server to read my Toggl credentials from `.env` and fail loudly at startup if they are missing, so I never debug a cryptic 401 mid-conversation.

**Why P1**: Nothing else in the feature functions without it.

**Acceptance Criteria**:

1. WHEN the server starts THEN it SHALL load `TOGGL_API_TOKEN` (required) and `TOGGL_WORKSPACE_ID` (required, positive integer) from the environment (including a `.env` file), and SHALL exit non-zero with a message naming each missing or invalid variable before registering any tool.
2. WHEN the server makes any Toggl call THEN it SHALL authenticate with HTTP Basic Auth using the API token as the username and the literal `api_token` as the password.
3. WHEN any error, log line, or tool result is emitted THEN it SHALL NOT contain the value of `TOGGL_API_TOKEN`.
4. WHEN the repository is checked out THEN it SHALL contain a committed `mcp/.env.sample` listing every variable with placeholder values, and `mcp/.env` SHALL be gitignored.

**Independent Test**: Launch with `TOGGL_API_TOKEN` unset — expect a non-zero exit naming that variable, and no MCP handshake. Launch with both set — expect a successful handshake and a tool list.

---

### P1: Read time entries by date range ⭐ MVP

**User Story**: As someone reviewing my week, I want to ask an agent for every entry between two dates and get back a compact list, so I can see what I tracked without opening Toggl.

**Why P1**: This is the primary use case named in the seed.

**Acceptance Criteria**:

1. WHEN `list_time_entries` is called with `start_date` and `end_date` THEN system SHALL issue exactly **one** `GET /api/v9/me/time_entries?start_date=…&end_date=…` request — never one request per day or per entry.
2. WHEN `start_date` or `end_date` is not a `YYYY-MM-DD` date or an RFC3339 timestamp, or `end_date` is earlier than `start_date` THEN system SHALL reject the call with a validation error naming the offending parameter, and SHALL NOT issue any Toggl request.
3. WHEN entries are returned THEN system SHALL emit, per entry, exactly these five fields and no others: `id`, `description`, `start`, `stop`, `project` — where `project` is the entry's `project_name`, or `null` when the entry has no project.
4. WHEN the response contains entries belonging to a workspace other than the configured (or per-call overridden) `workspace_id` THEN system SHALL omit them from the result.
5. WHEN the range contains no matching entries THEN system SHALL return an empty list as a **successful** result, not an error.
6. WHEN `get_time_entry` is called with an entry `id` THEN system SHALL issue one `GET /api/v9/me/time_entries/{id}` and return the same five-field curated shape.
7. WHEN `get_time_entry` is called with an id that does not exist or is not visible to the token THEN system SHALL return a structured not-found error, not an empty success.

**Independent Test**: Against a fake Toggl server, call `list_time_entries` for a 7-day range spanning 3 entries (one in a foreign workspace) — assert exactly one outbound request, and a 2-entry result carrying only the five curated fields.

---

### P1: Project cache with TTL and manual refresh ⭐ MVP

**User Story**: As the operator, I want the project list cached locally for about a week, so ticket-code matching does not spend one of my 30 hourly API calls on every single create.

**Why P1**: Without it, matching is unaffordable under the rate limit.

**Acceptance Criteria**:

1. WHEN a matching operation needs projects and no cache file exists THEN system SHALL issue one `GET /api/v9/me/projects?include_archived=false`, use the result, and write it to the cache file together with a fetch timestamp.
2. WHEN a cache file exists and its fetch timestamp is **less than 7 days old** THEN system SHALL use it and SHALL NOT issue any projects request.
3. WHEN a cache file exists and its fetch timestamp is **7 days or older** THEN system SHALL refetch and overwrite the cache before matching.
4. WHEN `refresh_projects` is called THEN system SHALL refetch and overwrite the cache regardless of the timestamp, and SHALL return the number of projects cached and the new fetch timestamp.
5. WHEN the cache file is missing, unreadable, malformed JSON, or lacks its timestamp field THEN system SHALL treat it as a cache miss and refetch — never crash and never throw the parse error to the agent.
6. WHEN the cache file cannot be written (permissions, read-only filesystem) THEN system SHALL log the failure to stderr, serve the freshly-fetched list from memory for the current call, and complete the tool call successfully.
7. WHEN the cache is stale and the refetch fails (network error, `429`, 5xx) **and** a stale cache is available THEN system SHALL match against the stale cache and include a `stale_cache` warning (with the cache's age and the underlying failure) in the tool result.
8. WHEN the cache is stale or absent, the refetch fails, **and** no cache is available at all THEN system SHALL return the underlying Toggl error, not a silent "no match".

**Independent Test**: With a fake Toggl server and a temp cache dir: first matching call writes the cache and makes one projects request; a second call makes zero; back-dating the timestamp 8 days makes the third call refetch; corrupting the file to `{`  makes the fourth refetch without error.

---

### P1: Ticket-code → project auto-matching ⭐ MVP

**User Story**: As someone logging `[JSP-10] Creating form`, I want the entry filed under the `JSP` project automatically, so I stop picking projects by hand and stop picking the wrong one.

**Why P1**: This is the differentiator over calling the Toggl API directly.

**Acceptance Criteria**:

1. WHEN a create or update supplies a `description` THEN system SHALL extract the ticket code using `^\[([A-Z][A-Z0-9]{1,9})-(\d+)\]\s*(.*)$` — the same pattern as `to-jira/internal/toggl/parse.go` — taking capture group 1 as the code (`JSP` from `[JSP-10] Creating form`).
2. WHEN comparing a candidate project THEN system SHALL strip a leading `[...]` bracket group and any immediately following whitespace from the project's literal `name`, and compare the remaining text to the extracted code with **exact, case-insensitive equality** — never substring, prefix, or fuzzy matching.
3. WHEN evaluating candidates THEN system SHALL consider only projects with `active: true`.
4. WHEN exactly one project matches THEN system SHALL attach that project's `id` to the entry and report the resolved project name in the tool result.
5. WHEN **more than one** project matches THEN system SHALL return a structured error naming the extracted code and listing every matching project's `id` and `name`, and SHALL NOT create or update anything.
6. WHEN **no** project matches THEN system SHALL return a structured error naming the extracted code and listing every active project's `id` and `name`, and SHALL NOT create or update anything.
7. WHEN an explicit `project_id` is supplied THEN system SHALL use it verbatim, SHALL skip code extraction and matching entirely, and SHALL NOT read the project cache.
8. WHEN the description does not match the pattern and no `project_id` is supplied THEN system SHALL proceed with no project attached and report `project: null` in the result.

**Independent Test**: With projects `[teachmeto.ai] JSP`, `[teachmeto.ai] OIQ`, `Staff Engineer Test`: `[JSP-10] x` resolves to the JSP project; `[XYZ-1] x` errors listing all three; adding a second `[other] jsp` project makes `[JSP-10] x` error listing exactly those two; passing `project_id` explicitly bypasses all of it.

---

### P1: Create a time entry ⭐ MVP

**User Story**: As someone who forgot to track something, I want to tell an agent "log 2-4pm yesterday on `[JSP-10] Creating form`" and have it appear in Toggl.

**Why P1**: Core CRUD; the write half of the feature.

**Acceptance Criteria**:

1. WHEN `create_time_entry` is called THEN it SHALL require `description`, `start`, and `stop` (RFC3339 UTC), and SHALL accept optional `project_id` and `workspace_id`.
2. WHEN `start` or `stop` is not valid RFC3339, or `stop` is not strictly after `start` THEN system SHALL reject the call with a validation error and SHALL NOT issue any Toggl request.
3. WHEN validation passes THEN system SHALL resolve the project per the matching story, then issue one `POST /api/v9/workspaces/{workspace_id}/time_entries` with `start`, `stop`, `description`, the resolved `project_id` (omitted when null), `workspace_id`, a computed `duration` in seconds, and a fixed `created_with` identifier.
4. WHEN `workspace_id` is not supplied THEN system SHALL use `TOGGL_WORKSPACE_ID`; when it is supplied THEN system SHALL use the supplied value for that call only.
5. WHEN the create succeeds THEN system SHALL return the created entry in the same five-field curated shape as reads.
6. WHEN project matching fails (zero or multiple matches) THEN system SHALL return that matching error and SHALL NOT issue the `POST` — no partially-created entry.

**Independent Test**: Create `[JSP-10] Creating form` for a 2-hour window — assert the outbound `POST` body carries the JSP project id and `duration: 7200`, and the result echoes the curated shape. Create with an ambiguous code — assert zero `POST` requests.

---

### P1: Update a time entry ⭐ MVP

**User Story**: As someone who mistyped an entry, I want to correct its description or times without deleting and recreating it.

**Why P1**: Named in the seed alongside create and delete.

**Acceptance Criteria**:

1. WHEN `update_time_entry` is called THEN it SHALL require `id` and at least one of `description`, `start`, `stop`, or `project_id`, and SHALL reject a call supplying only `id`.
2. WHEN the call is valid THEN system SHALL fetch the current entry, merge the supplied fields over it, and issue one `PUT /api/v9/workspaces/{workspace_id}/time_entries/{id}` with the merged entry — so that fields the caller omitted are preserved, not cleared.
3. WHEN `description` is among the supplied fields and no explicit `project_id` is supplied THEN system SHALL re-run ticket-code matching against the **new** description and update the entry's project accordingly.
4. WHEN `description` is not supplied THEN system SHALL leave the entry's existing project untouched and SHALL NOT read the project cache.
5. WHEN the merged `start`/`stop` pair would leave `stop` not strictly after `start` THEN system SHALL reject the call and SHALL NOT issue the `PUT`.
6. WHEN the update succeeds THEN system SHALL return the updated entry in the curated five-field shape.

**Independent Test**: Update only `description` from `[JSP-10] a` to `[OIQ-3] b` — assert the outbound `PUT` carries the OIQ project id, the original `start`/`stop` unchanged, and that the fetch-then-put pair is exactly two requests.

---

### P1: Delete a time entry ⭐ MVP

**User Story**: As someone cleaning up a bad tracking session, I want to remove an entry without opening Toggl.

**Why P1**: Named in the seed; the third leg of core CRUD.

**Acceptance Criteria**:

1. WHEN `delete_time_entry` is called with an `id` THEN system SHALL issue one `DELETE /api/v9/workspaces/{workspace_id}/time_entries/{id}` and return a success result naming the deleted id.
2. WHEN the entry does not exist or is not accessible THEN system SHALL return a structured not-found error naming the id — not a silent success.

**Independent Test**: Delete an existing id — assert exactly one outbound `DELETE` and a success result naming it. Delete an id the fake server 404s — assert a structured not-found error, not a success.

---

### P1: Transparent errors and clean stdio transport ⭐ MVP

**User Story**: As the agent driving this server, I want Toggl's real errors — especially rate limiting — passed through unchanged, so I can tell the user to wait rather than retrying blindly.

**Why P1**: Q7 made unmediated error surfacing an explicit design choice, and stdout purity is a hard correctness requirement for stdio MCP.

**Acceptance Criteria**:

1. WHEN Toggl returns `429` THEN system SHALL return a structured error carrying the status, Toggl's response body, and the `Retry-After` header when present, and SHALL NOT sleep, retry, back off, or pre-emptively throttle any request.
2. WHEN Toggl returns any other non-2xx status THEN system SHALL return a structured error carrying the status, the request's method and path, and the response body.
3. WHEN a Toggl request fails at the network level or times out THEN system SHALL return a structured error identifying the failed operation — never an empty result that reads as "nothing found".
4. WHEN the server emits any log, diagnostic, or warning THEN it SHALL write it to **stderr only**; stdout SHALL carry nothing but MCP protocol frames.
5. WHEN any tool is invoked with arguments that fail its input schema THEN system SHALL return a validation error identifying the offending field, and SHALL NOT issue any Toggl request.

**Independent Test**: Point the client at a fake Toggl returning `429` with `Retry-After: 900` — assert the tool result carries status, body, and 900, that no second request is made, and that the process wrote zero non-protocol bytes to stdout.

---

## Edge Cases

- WHEN a project's `name` is exactly `[teachmeto.ai]` with nothing after the bracket THEN system SHALL treat its stripped remainder as empty and never match any code (an empty remainder can never equal a `[A-Z][A-Z0-9]{1,9}` code).
- WHEN a description is `[JSP-10]` with no trailing text THEN system SHALL still extract `JSP` and match normally — the regex's trailing group is allowed to be empty.
- WHEN a description contains a bracket tag that is not at the start (e.g. `Fixing [JSP-10] today`) THEN system SHALL treat it as untagged — the pattern is anchored, and no fuzzy scan is performed.
- WHEN a lowercase tag such as `[jsp-10]` is used THEN system SHALL treat it as untagged — the pattern requires uppercase, matching `to-jira`'s existing behavior exactly.
- WHEN `list_time_entries` is called with a range spanning many months THEN system SHALL still issue exactly one request and return whatever Toggl returns; no client-side pagination or range splitting is performed (no documented result cap was found in Toggl's spec — flagged as unverified).
- WHEN two tool calls race to refresh the project cache THEN the cache file SHALL never be left half-written — the write is atomic (temp file + rename), and a torn read degrades to a cache miss (TEM-10) rather than a crash.
- WHEN the configured `TOGGL_WORKSPACE_ID` does not match any workspace the token can access THEN reads SHALL return an empty list and writes SHALL surface Toggl's own 403/404 — no special-case handling.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| TEM-01 | P1: Configuration — required env vars validated at startup | Design | Implementing |
| TEM-02 | P1: Configuration — Basic Auth with `api_token`, token never logged, `.env.sample` committed | Design | Implementing |
| TEM-03 | P1: Read — date range in exactly one API call | Design | Implementing |
| TEM-04 | P1: Read — date/range input validation | Design | Implementing |
| TEM-05 | P1: Read — curated five-field output shape | Design | Implementing |
| TEM-06 | P1: Read — client-side workspace filtering; empty range is success | Design | Implementing |
| TEM-07 | P1: Read — get single entry, not-found is a structured error | Design | Implementing |
| TEM-08 | P1: Cache — fetch-and-persist projects on miss | Design | Implementing |
| TEM-09 | P1: Cache — 7-day TTL; `refresh_projects` forces refresh | Design | Implementing |
| TEM-10 | P1: Cache — corrupt/unreadable cache degrades to a miss, never crashes | Design | Implementing |
| TEM-11 | P1: Cache — unwritable cache still serves from memory | Design | Implementing |
| TEM-12 | P1: Cache — stale-cache fallback with `stale_cache` warning; hard error when no cache exists | Design | Implementing |
| TEM-13 | P1: Matching — extract code via `to-jira`'s `[SLUG-NUMBER]` pattern | Design | Implementing |
| TEM-14 | P1: Matching — strip `[...]` prefix, exact case-insensitive equality, active projects only | Design | Implementing |
| TEM-15 | P1: Matching — zero or multiple matches ⇒ structured error listing candidates, no write | Design | Implementing |
| TEM-16 | P1: Matching — explicit `project_id` bypasses matching and the cache | Design | Implementing |
| TEM-17 | P1: Matching — untagged description ⇒ no project, reported as `project: null` | Design | Implementing |
| TEM-18 | P1: Create — input contract and RFC3339 / start-before-stop validation | Design | Implementing |
| TEM-19 | P1: Create — single `POST` with resolved project, duration, `created_with` | Design | Implementing |
| TEM-20 | P1: Create — workspace default with per-call override | Design | Implementing |
| TEM-21 | P1: Update — read-modify-write preserves omitted fields | Design | Implementing |
| TEM-22 | P1: Update — re-match project on description change only | Design | Implementing |
| TEM-23 | P1: Delete — single delete; not-found is a structured error | Design | Implementing |
| TEM-24 | P1: Errors — `429` and non-2xx surfaced verbatim; no retry/throttle; stderr-only logging; schema validation before any request | Design | Implementing |

**ID format:** `TEM-NN`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 24 total, 24 mapped to Design, 0 unmapped

---

## Implicit-Requirement Dimensions Sweep

Large scope — every dimension resolves to a requirement or an explicit `N/A because …`.

| Dimension | Resolution |
| --- | --- |
| Auth boundaries & rate limits | TEM-01, TEM-02, TEM-24 — Basic Auth via env token, no proactive throttling, `429` surfaced with `Retry-After`. |
| Concurrency / ordering | TEM-10 + Edge Cases — atomic cache write (temp + rename); a torn read degrades to a cache miss. Otherwise **N/A because** stdio serves one local client and no tool mutates shared in-process state. |
| Data lifecycle / expiry | TEM-09, TEM-12 — 7-day TTL, manual refresh, stale-cache fallback. No other persisted data exists. |
| External-dependency failure | TEM-12, TEM-24 — Toggl unreachable/5xx/timeout returns a structured error; project matching degrades to the stale cache rather than going offline. |
| Failure / partial-failure states | TEM-15/TEM-19 keep create atomic (matching failure means no `POST` at all) and TEM-21 keeps update atomic (a failed merge means no `PUT`). TEM-12 is the one deliberate partial-success path: a stale cache serves the call with a `stale_cache` warning. **N/A** for multi-entity partial failure because every tool performs exactly one logical write — bulk delete is out of scope, and an agent looping `delete_time_entry` sees each call's own result. |
| Idempotency / retry / duplicate handling | TEM-23 (repeat delete ⇒ explicit not-found), TEM-24 (**no** automatic retry — Q7). **N/A** for dedup keys because every write targets an explicit Toggl id. |
| Input validation & bounds | TEM-04, TEM-18, TEM-21, TEM-24 — date formats, start-before-stop, schema validation before any outbound call. |
| Observability | TEM-24 — structured stderr logging; stdout reserved for MCP frames. OTel is out of scope (see Out of Scope, AD-003 rationale). |
| State-transition integrity | **N/A because** the server has no state machine. Its only state is the project cache, whose sole transition (fresh ⇄ stale ⇄ absent) is fully covered by TEM-08 through TEM-12. |

---

## Success Criteria

- [ ] Asking an agent "what did I track between 2026-08-18 and 2026-08-24?" returns the full curated list from a single Toggl API call.
- [ ] Creating `[JSP-10] Creating form` attaches the correct project with zero human project selection, and creating `[XYZ-9] …` fails with a candidate list instead of guessing.
- [ ] After the first project fetch, a full day of create/update calls consumes **zero** additional projects requests.
- [ ] A `429` from Toggl reaches the agent with its status and `Retry-After` intact, and the server issues no retry of its own.
- [ ] The whole test suite runs with **no live network calls** — Toggl is always a local fake — matching `to-jira`'s hard project constraint (`docs/codebase/TESTING.md`).
- [ ] No database, no Redis, no networked store: the only persisted artifact is one JSON cache file, and deleting it costs exactly one API call to rebuild.
