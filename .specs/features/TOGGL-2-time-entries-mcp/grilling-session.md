# Grilling Session: TOGGL-2 — Time Entries MCP

## Seed

TypeScript, stateless MCP server exposing Toggl Track's Time Entries API
(https://engineering.toggl.com/docs/track/api/time_entries/) for read/create/update/delete.
No running-tracker support — all operations specify explicit start/end times. Primary use
case is extracting time entries by date; other usages may come later. User wanted to
leverage Toggl's OpenAPI codegen CLI (https://engineering.toggl.com/docs/track/openapi/) to
avoid hand-writing the API client, but flagged a hard constraint: only 30 requests/hour
available against the Toggl API, so the design must minimize API calls — user wanted to be
told if codegen's flexibility can't fit within that cap. Repo is an existing monorepo
(`to-jira/` Go service already lives here, stateless-by-design per `.specs/STATE.md` AD-001);
this new MCP is a sibling package.

## Background Research (dispatched during grilling, not user input)

### Codegen feasibility vs. the 30 req/hour cap

- Toggl's spec is **Swagger 2.0** (not OpenAPI 3.x), hosted as static JSON at
  `https://engineering.toggl.com/assets/files/api-608a9fccdf09a653b6842ed1793cfc4c.json`
  (`basePath: /api/v9`, `host: localhost:8080` — must override base URL).
- Toggl's own docs recommend legacy `swagger-codegen` (Java/Docker) — no TS-native codegen
  guidance from Toggl itself.
- `GET /api/v9/me/time_entries` supports `start_date`/`end_date` (or `since`/`before`) query
  params in a single request — the primary "extract by date" use case is a single API call,
  not one-call-per-entry. No documented range/result cap found (unverified: whether an
  undocumented server-side cap exists on very wide ranges).
- `openapi-typescript`/`openapi-fetch` (types + optional fetch wrapper) do **not** support
  Swagger 2.0 directly (v3.0+ only) — would need a one-time `swagger2openapi` conversion.
- `openapi-typescript-codegen` is **deprecated** by its own author in favor of
  `@hey-api/openapi-ts`.
- `@hey-api/openapi-ts` parses Swagger 2.0 **natively** (no conversion needed) and its
  `client-fetch`/`client-axios` plugins expose documented request interceptors
  (`onRequest` / Axios `interceptors.request.use`) — technically capable of a quota guard.
- Conclusion given to the user: **codegen is not ruled out by the rate-limit constraint** —
  contrary to the initial worry — but for the small endpoint surface this MCP needs
  (~8-10 endpoints), a hand-written thin client with types-only codegen is simpler and gives
  more direct control, with no generated-client machinery to learn. User confirmed this
  choice (see Q15 below).

### Toggl Projects API schema (verified against raw Swagger JSON, not the UI)

- `GET /api/v9/workspaces/{workspace_id}/projects` returns objects with **separate**
  `id`, `name`, `client_id` (nullable), `client_name`, `active`, `status`, `workspace_id`
  fields — the API does not embed client name into `name` automatically.
- A separate `GET /api/v9/workspaces/{workspace_id}/clients` endpoint exists
  (`models.Client`: `id`, `name`, `wid`, `archived`, ...).
- **User confirmed empirically** (Q12 below) that for their account, the bracketed client
  prefix (e.g. `[teachmeto.ai] JSP`) is literally stored in the project's own `name` field —
  not a UI-only decoration of a separate `client_name` field. Matching logic must strip a
  leading `[...] ` prefix from `name` before comparing to an extracted ticket code.

### Existing convention in this monorepo

`to-jira/internal/toggl/parse.go` already parses Toggl entry descriptions tagged
`[SLUG-NUMBER] text` via `^\[([A-Z][A-Z0-9]{1,9})-(\d+)\]\s*(.*)$`. TOGGL-2 reuses this exact
pattern for extracting the ticket code (e.g. `JSP` from `[JSP-10] Creating form`) for
consistency with the established repo convention.

## Round 1

❓ **Q1** - Distribution model: stdio vs HTTP/SSE remote.
➡️ Recommended stdio. **Answer: stdio.**

❓ **Q2** - Package location: flat sibling directory vs npm workspaces monorepo.
➡️ Recommended flat sibling `toggl-mcp/`. **Answer: flat sibling, but named `mcp/`
(folder name is just `mcp`, not `toggl-mcp`).**

❓ **Q3** - Workspace scoping: default env var vs required per-call `workspace_id`.
➡️ Recommended default via `TOGGL_WORKSPACE_ID` env var with optional override.
**Answer: confirmed — plus a whole new requirement surfaced here:** ticket-code →
project auto-matching. User's Toggl projects are named like `[teachmeto.ai] JSP`,
`[teachmeto.ai] OIQ`, `[Santo Cartão] E-commerce`, `Staff Engineer Test` — the
`[Client] CODE` pattern is **not** consistent across all projects. When a time entry
description looks like `[JSP-10] Creating form`, the MCP must extract the `JSP` code and
match it to the correct project. Deduplication/ambiguity must **never** be silently
resolved by the MCP — it must return an error the agent can surface to the user.

❓ **Q4** - Tool/endpoint scope: single-entry CRUD only, or also bulk PATCH/DELETE.
➡️ Recommended include both bulk PATCH and DELETE.
**Answer: bulk DELETE only — no bulk PATCH for now.**

❓ **Q5** - Read output shape: curated vs raw passthrough.
➡️ Recommended curated, but keep all fields available.
**Answer: curated, and narrower than recommended — only ID, Description, Start, Stop,
Project.**

❓ **Q6** - Auth mechanism: API token vs email/password.
➡️ Recommended API token via env var.
**Answer: confirmed — `TOGGL_API_TOKEN` living in `.env`; create a committed
`.env.sample` with the initial state.**

❓ **Q7** - Rate-limit enforcement: in-memory rolling counter (fail fast) vs free calls +
surface Toggl's 429.
➡️ Recommended in-memory rolling-window counter.
**Answer: make calls freely, return 429 as-is — no proactive throttling.**

❓ **Q8** - Response caching for time-entry reads: skip vs short-lived in-memory cache.
➡️ Recommended skip for v1.
**Answer: confirmed — no caching.**

## Round 2

Prompted by Q3's answer (ticket-code → project matching), after confirming
`to-jira/internal/toggl/parse.go`'s existing `[SLUG-NUMBER]` convention directly from the
repo, and dispatching research into the Toggl Projects API schema.

❓ **Q9** - New read scope: is Projects (+Clients) reading in scope for TOGGL-2?
➡️ Recommended yes — the matching feature is unusable without it.
**Answer: confirmed, in scope.**

❓ (cache question) - Cache Projects/Clients list: in-memory per-process vs always fetch
fresh vs (user-proposed) Redis shared via the k3d local-env container, TTL ~1 week+.
➡️ Initially recommended in-memory per-process.
**Answer: user asked for Redis** (see Round 3 — this was challenged and revised).

❓ **Match scope** - Ticket-code matching on create only, or also update.
➡️ Recommended both.
**Answer: confirmed — both create and update.**

❓ **Tool interface** - Auto-resolve project from description by default (with override
param) vs explicit two-step resolve-then-create.
➡️ Recommended auto-resolve with override.
**Answer: confirmed — auto-resolve by default, optional override.**

## Round 3

❓ **Q10** - Pushed back on the Redis proposal: it directly contradicts this project's own
"completely stateless" requirement and this repo's own `to-jira` AD-001 precedent
(no DB/persistence), and adds a new external dependency + failure mode (k3d Redis
unreachable breaks project-matching entirely) for a benefit a local file cache gives just
as well.
➡️ Recommended local JSON file cache with a ~1 week TTL instead of Redis.
**Answer: confirmed — local JSON file cache. No Redis.**

❓ **Q11** - Manual `refresh_projects` tool to force an early cache refresh?
➡️ Recommended yes.
**Answer: confirmed — add it.**

❓ **Q12** - Empirical question only the user could answer: is the `[teachmeto.ai]`
bracket literally part of the Toggl project's `name` field, or a UI-only decoration of a
separate `client_name` field (confirmed by research to exist as a distinct API field)?
➡️ No safe default — asked rather than guessed, and rather than burn one of the user's
rate-limited API calls to check directly.
**Answer: confirmed — it's literally part of the project's `name`, including the
brackets.** Matching logic strips a leading `[...] ` prefix before comparing to the
extracted ticket code.

❓ **Q15** - API client generation approach: hand-written thin client + types-only codegen
(via `openapi-typescript`, after a one-time Swagger 2.0→3.0 conversion) vs full runtime
codegen via `@hey-api/openapi-ts` (natively supports Swagger 2.0, has a documented
`onRequest` interceptor that could satisfy the quota-guard requirement).
➡️ Recommended hand-written client + types-only codegen — codegen is not ruled out by
the rate limit (worth telling the user plainly, since they asked to be told), but for
~8-10 endpoints a generated runtime client is more machinery than the problem warrants.
**Answer: confirmed — hand-written client, types-only codegen.**

## Final Shared Understanding (confirmed by user)

**Architecture**
- TypeScript MCP server, stdio transport, new sibling directory `mcp/` at repo root (flat,
  no npm workspaces)
- Stateless in the same sense `to-jira` uses the word (no DB) — the one exception is a local
  JSON file cache for Projects/Clients

**Toggl API client**
- Hand-written thin `fetch` wrapper; TypeScript types generated via `openapi-typescript`
  (after a one-time Swagger 2.0→3.0 spec conversion) — no runtime codegen
- Auth: `TOGGL_API_TOKEN` (Basic Auth, API-token style) via env var; `.env.sample` committed
- Default `TOGGL_WORKSPACE_ID` via env var, optional per-call override
- No client-side rate throttling — calls fire freely, Toggl's `429` surfaced straight back
  to Claude

**Time Entries tools**
- List/get by date range (primary use case), get single, create, update, delete single,
  bulk delete (no bulk patch)
- Reads return a curated shape only: id, description, start, stop, project
- No caching of time-entry reads

**Ticket-code → project matching** (new scope beyond pure Time Entries)
- Reuses `to-jira`'s existing convention: `^\[([A-Z][A-Z0-9]{1,9})-(\d+)\]\s*(.*)$` to
  extract the code from a description like `[JSP-10] Creating form`
- Matches the code against each Toggl project's literal `name` by stripping a leading
  `[...] ` prefix and comparing the remainder case-insensitively
- Applies on both `create_time_entry` and `update_time_entry`; auto-resolves by default,
  with an optional explicit `project_id`/`project_name` param to override or recover from
  an error
- No match or multiple matches → structured error listing candidate projects, never
  silently guessed
- Projects (+ Clients if needed) read via Toggl's Projects/Clients endpoints, cached in a
  local JSON file with a ~1 week TTL, plus a manual `refresh_projects` tool to force an
  early refresh
