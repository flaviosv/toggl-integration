# Time Entries MCP Specification

**Feature:** TOGGL-2 · **Scope size:** Small (single TS package, one read-only MCP tool, Toggl API client, project-name cache)
**Source of scope:** `.specs/features/TOGGL-2-time-entries-mcp/grilling-session.md` (3 rounds, 15 questions, user-confirmed) — **superseded in part by the 2026-08-26 revision below.**

## Revision (2026-08-26): scope reduced to read-only listing

After the feature was fully built and reviewed (create/update/delete, ticket-code project
auto-matching, `get_time_entry`, `refresh_projects`), the user asked to simplify: keep only
listing time entries by date range, drop every write path and the matching engine it existed to
serve. This revision reflects that decision as the current, authoritative scope. The rest of this
document describes the system **as it stands after the cut**, not as originally built — the
grilling session above (Q3–Q4, Q9–Q17ish) is kept for history but no longer describes current
scope where it conflicts with this section.

**Dropped:** `create_time_entry`, `update_time_entry`, `delete_time_entry`, `get_time_entry`,
`refresh_projects`, and the ticket-code → project matching engine (`extractTicketCode` /
`resolveProject`) that only create/update needed. Superseded requirement IDs TEM-07, TEM-09 (the
`refresh_projects` half), TEM-13 through TEM-23 are recorded as **Dropped** in Requirement
Traceability below rather than renumbered, to keep the audit trail of what was built and then
removed intact.

**Kept:** `list_time_entries` (date-range read) and the project-name cache it relies on to render
a curated `project` field, since dropping the cache would mean either an extra API call per list
or a bare project id instead of a name — no simplification benefit, real loss of usefulness.

## Problem Statement

Reading Toggl time entries today means opening the Toggl web UI by hand — there is no way for an
agent to answer "what did I track last week?" without a human clicking through. TOGGL-2 adds a
stateless TypeScript MCP server (stdio) that gives Claude read-only access to Toggl time entries
by date range, curated to the fields that matter for reviewing time, not managing it.

## Goals

- [ ] An agent can list every time entry in an arbitrary date range in **one** Toggl API call, receiving a curated shape (id, description, start, stop, project).
- [ ] The server stays stateless in the `to-jira` sense (AD-001): no database, no networked store. The only persisted artifact is a local JSON cache of Projects, used solely to resolve a project's name for display.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| --- | --- |
| Create, update, or delete time entries | User request (2026-08-26 revision) — this server is read-only. The write paths, and the matching engine that only they needed, are removed entirely rather than left dormant. |
| `get_time_entry` (single-entry read) | User request (2026-08-26 revision) — `list_time_entries` with a narrow date range covers the single-entry case; a second read tool added no value once writes were cut. |
| Ticket-code → project auto-matching | User request (2026-08-26 revision) — existed only to resolve a project on create/update. With no writes, there is nothing left to resolve a project *for*; the list output still shows each entry's existing project name via the cache, unchanged. |
| `refresh_projects` (manual cache-bust tool) | User request (2026-08-26 revision) — a second tool to force-refresh a cache that already self-refreshes on a 7-day TTL added surface without a confirmed need, once the tool count was being minimized. |
| CI pipeline | Consistent with `to-jira`'s explicit deferral (`.specs/features/to-jira/spec.md` Out of Scope). |
| Client-side rate throttling, retry, or backoff | Calls fire freely; Toggl's `429` is surfaced verbatim to the agent, which decides what to do. |
| HTTP / SSE remote transport | stdio only, single local operator. |
| Multi-workspace / multi-account support | One default workspace via `TOGGL_WORKSPACE_ID`, with an optional per-call override. |
| npm workspaces / monorepo build tooling | Flat sibling directory `mcp/`, self-contained `package.json`. |
| OpenTelemetry instrumentation | AD-003 scopes the OTel pattern to long-lived *services*. This is a short-lived stdio child process with no collector to export to; stderr logging is the whole observability story (see TEM-24). |
| Redis or any shared / networked cache | Contradicts the project's own statelessness requirement and AD-001, and adds a failure mode a local file cache does not. |
| Running / live-timer support (start, stop, `current`) | Every entry is read as Toggl already recorded it; there is no timer control here at all. |
| Tags, billable flags, task assignment on entries | Not requested; the curated read shape deliberately excludes them. |
| Time-entry response caching | Reads always hit Toggl, so the agent never reasons about stale time data. |

---

## Assumptions & Open Questions

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --- | --- | --- | --- |
| Project cache location | `~/.cache/toggl-mcp/projects.json`, overridable via `TOGGL_CACHE_PATH` | An MCP server is spawned by the client with an unpredictable working directory, so a repo-relative path is unreliable. Keeps generated state out of the git tree. | n |
| Project cache TTL | Fixed **7 days**, not configurable by env var | Matches the original grilling answer ("~1 week"). With `refresh_projects` dropped, the TTL is the only cache-freshness lever, and self-refreshing on read is simpler than adding a manual bust tool back just to make it configurable. | n |
| Stale cache + Toggl unreachable | Serve the **stale** cache and attach a `stale_cache` warning to the result, rather than failing the call | With a 30 req/hour ceiling, a `429` during a project refresh must not take the whole list call offline. Degrading loudly beats failing hard. | n |
| Read scoping to the configured workspace | `list_time_entries` reads via `/me/time_entries` (no workspace path param) and then **filters client-side** to `TOGGL_WORKSPACE_ID` (or the per-call override) | `/me/*` returns entries across every workspace the token can see; without the filter, "the workspace" would be ambiguous. | n |

**Open questions:** none — all resolved or logged above.

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

**Why P1**: This is the primary — and, after the 2026-08-26 revision, only — use case.

**Acceptance Criteria**:

1. WHEN `list_time_entries` is called with `start_date` and `end_date` THEN system SHALL issue exactly **one** `GET /api/v9/me/time_entries?start_date=…&end_date=…` request — never one request per day or per entry.
2. WHEN `start_date` or `end_date` is not a `YYYY-MM-DD` date or an RFC3339 timestamp, or `end_date` is earlier than `start_date` THEN system SHALL reject the call with a validation error naming the offending parameter, and SHALL NOT issue any Toggl request.
3. WHEN entries are returned THEN system SHALL emit, per entry, exactly these five fields and no others: `id`, `description`, `start`, `stop`, `project` — where `project` is the entry's `project_name`, or resolved from the project cache by `project_id`, or `null` when the entry has no project.
4. WHEN the response contains entries belonging to a workspace other than the configured (or per-call overridden) `workspace_id` THEN system SHALL omit them from the result.
5. WHEN the range contains no matching entries THEN system SHALL return an empty list as a **successful** result, not an error.

**Independent Test**: Against a fake Toggl server, call `list_time_entries` for a 7-day range spanning 3 entries (one in a foreign workspace) — assert exactly one outbound request, and a 2-entry result carrying only the five curated fields.

---

### P1: Project-name cache with TTL ⭐ MVP

**User Story**: As the operator, I want project names cached locally for about a week, so listing entries with projects does not spend one of my 30 hourly API calls on every single list.

**Why P1**: Without it, every list call with a project-bearing entry costs two Toggl requests instead of one on a warm cache.

**Acceptance Criteria**:

1. WHEN a list needs a project name and no cache file exists THEN system SHALL issue one `GET /api/v9/me/projects?include_archived=false`, use the result, and write it to the cache file together with a fetch timestamp.
2. WHEN a cache file exists and its fetch timestamp is **less than 7 days old** THEN system SHALL use it and SHALL NOT issue any projects request.
3. WHEN a cache file exists and its fetch timestamp is **7 days or older** THEN system SHALL refetch and overwrite the cache before resolving project names.
4. WHEN the cache file is missing, unreadable, malformed JSON, or lacks its timestamp field THEN system SHALL treat it as a cache miss and refetch — never crash and never throw the parse error to the agent.
5. WHEN the cache file cannot be written (permissions, read-only filesystem) THEN system SHALL log the failure to stderr, serve the freshly-fetched list from memory for the current call, and complete the tool call successfully.
6. WHEN the cache is stale and the refetch fails (network error, `429`, 5xx) **and** a stale cache is available THEN system SHALL resolve project names against the stale cache and include a `stale_cache` warning (with the cache's age and the underlying failure) in the tool result.
7. WHEN the cache is stale or absent, the refetch fails, **and** no cache is available at all THEN system SHALL return the underlying Toggl error, not a silent "no project name".

**Independent Test**: With a fake Toggl server and a temp cache dir: first list call with a project-bearing entry writes the cache and makes one projects request; a second call makes zero; back-dating the timestamp 8 days makes the third call refetch; corrupting the file to `{` makes the fourth refetch without error.

---

### P1: Transparent errors and clean stdio transport ⭐ MVP

**User Story**: As the agent driving this server, I want Toggl's real errors — especially rate limiting — passed through unchanged, so I can tell the user to wait rather than retrying blindly.

**Why P1**: Unmediated error surfacing is an explicit design choice, and stdout purity is a hard correctness requirement for stdio MCP.

**Acceptance Criteria**:

1. WHEN Toggl returns `429` THEN system SHALL return a structured error carrying the status, Toggl's response body, and the `Retry-After` header when present, and SHALL NOT sleep, retry, back off, or pre-emptively throttle any request.
2. WHEN Toggl returns any other non-2xx status THEN system SHALL return a structured error carrying the status, the request's method and path, and the response body.
3. WHEN a Toggl request fails at the network level or times out THEN system SHALL return a structured error identifying the failed operation — never an empty result that reads as "nothing found".
4. WHEN the server emits any log, diagnostic, or warning THEN it SHALL write it to **stderr only**; stdout SHALL carry nothing but MCP protocol frames.
5. WHEN the tool is invoked with arguments that fail its input schema THEN system SHALL return a validation error identifying the offending field, and SHALL NOT issue any Toggl request.

**Independent Test**: Point the client at a fake Toggl returning `429` with `Retry-After: 900` — assert the tool result carries status, body, and 900, that no second request is made, and that the process wrote zero non-protocol bytes to stdout.

---

## Edge Cases

- WHEN `list_time_entries` is called with a range spanning many months THEN system SHALL still issue exactly one request and return whatever Toggl returns; no client-side pagination or range splitting is performed (no documented result cap was found in Toggl's spec — flagged as unverified).
- WHEN two tool calls race to refresh the project cache THEN the cache file SHALL never be left half-written — the write is atomic (temp file + rename), and a torn read degrades to a cache miss (TEM-10) rather than a crash.
- WHEN the configured `TOGGL_WORKSPACE_ID` does not match any workspace the token can access THEN the call SHALL return an empty list — no special-case handling.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| TEM-01 | P1: Configuration — required env vars validated at startup | Design | Verified |
| TEM-02 | P1: Configuration — Basic Auth with `api_token`, token never logged, `.env.sample` committed | Design | Verified |
| TEM-03 | P1: Read — date range in exactly one API call | Design | Verified |
| TEM-04 | P1: Read — date/range input validation | Design | Verified |
| TEM-05 | P1: Read — curated five-field output shape | Design | Verified |
| TEM-06 | P1: Read — client-side workspace filtering; empty range is success | Design | Verified |
| TEM-07 | ~~P1: Read — get single entry~~ | — | **Dropped** (2026-08-26 revision — `get_time_entry` removed) |
| TEM-08 | P1: Cache — fetch-and-persist projects on miss | Design | Verified |
| TEM-09 | P1: Cache — 7-day TTL | Design | Verified (the `refresh_projects` half of this requirement is **Dropped** — tool removed) |
| TEM-10 | P1: Cache — corrupt/unreadable cache degrades to a miss, never crashes | Design | Verified |
| TEM-11 | P1: Cache — unwritable cache still serves from memory | Design | Verified |
| TEM-12 | P1: Cache — stale-cache fallback with `stale_cache` warning; hard error when no cache exists | Design | Verified |
| TEM-13 | ~~P1: Matching — extract ticket code~~ | — | **Dropped** (2026-08-26 revision — matching engine removed, no writes left to serve) |
| TEM-14 | ~~P1: Matching — strip prefix, exact equality~~ | — | **Dropped** |
| TEM-15 | ~~P1: Matching — zero/multiple matches error~~ | — | **Dropped** |
| TEM-16 | ~~P1: Matching — explicit `project_id` bypass~~ | — | **Dropped** |
| TEM-17 | ~~P1: Matching — untagged description~~ | — | **Dropped** |
| TEM-18 | ~~P1: Create — input contract and validation~~ | — | **Dropped** (2026-08-26 revision — `create_time_entry` removed) |
| TEM-19 | ~~P1: Create — single `POST`~~ | — | **Dropped** |
| TEM-20 | ~~P1: Create — workspace default with override~~ | — | **Dropped** |
| TEM-21 | ~~P1: Update — read-modify-write~~ | — | **Dropped** (2026-08-26 revision — `update_time_entry` removed) |
| TEM-22 | ~~P1: Update — re-match on description change~~ | — | **Dropped** |
| TEM-23 | ~~P1: Delete — single delete~~ | — | **Dropped** (2026-08-26 revision — `delete_time_entry` removed) |
| TEM-24 | P1: Errors — `429` and non-2xx surfaced verbatim; no retry/throttle; stderr-only logging; schema validation before any request | Design | Verified |

**ID format:** `TEM-NN`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified → Dropped

**Coverage:** 11 active (TEM-01–06, 08–12, 24), 13 dropped (TEM-07, TEM-13–23), 24 total historically defined

---

## Success Criteria

- [ ] Asking an agent "what did I track between 2026-08-18 and 2026-08-24?" returns the full curated list from a single Toggl API call.
- [ ] After the first project fetch, a full day of list calls consumes **zero** additional projects requests.
- [ ] A `429` from Toggl reaches the agent with its status and `Retry-After` intact, and the server issues no retry of its own.
- [ ] The whole test suite runs with **no live network calls** — Toggl is always a local fake — matching `to-jira`'s hard project constraint (`docs/codebase/TESTING.md`).
- [ ] No database, no Redis, no networked store: the only persisted artifact is one JSON cache file, and deleting it costs exactly one API call to rebuild.
- [ ] Exactly one tool is registered: `list_time_entries`.
