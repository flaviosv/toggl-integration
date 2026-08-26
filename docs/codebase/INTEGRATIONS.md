# External Integrations

## Integrations

**Toggl Track (webhook source, consumed by `to-jira`):**

- Type: Third-party time-tracking API (webhook source)
- Purpose: Fires real-time events when a time entry is created, updated, or deleted, driving the entire sync pipeline
- Data flow: Inbound only
- Protocol: Webhooks (HTTPS POST, JSON body, HMAC-SHA256 signed)
- Location: `to-jira/internal/webhook/handler.go` (entrypoint), `to-jira/internal/toggl/` (payload types/parsing)
- Authentication: Shared-secret HMAC-SHA256 signature verification (`TOGGL_WEBHOOK_SECRET`, `X-Webhook-Signature-256` header) — Toggl authenticates itself to `to-jira`; `to-jira` makes no outbound calls to Toggl

**Toggl Track (REST API, consumed by `mcp`):**

- Type: Third-party time-tracking API (outbound REST client)
- Purpose: Read-only source for the `list_time_entries` MCP tool — time entries and project names
- Data flow: Outbound only
- Protocol: REST (Toggl Track API v9)
- Location: `mcp/src/toggl/client.ts` (`TogglClient`)
- Authentication: HTTP Basic Auth — API token as username, literal string `api_token` as password (`TOGGL_API_TOKEN`)

**JIRA Cloud:**

- Type: Third-party issue tracker API (worklog target)
- Purpose: Receives worklog create/update/delete calls that mirror synced Toggl entries
- Data flow: Outbound only
- Protocol: REST (JIRA Cloud REST API v3)
- Location: `to-jira/internal/jira/`
- Authentication: HTTP Basic Auth (`JIRA_EMAIL` + `JIRA_API_TOKEN`), set per-request in `jira.Client.do`

**OpenTelemetry Collector (optional, not yet deployed):**

- Type: Observability backend
- Purpose: Receives traces and metrics when configured
- Data flow: Outbound only
- Protocol: OTLP over HTTP
- Location: `to-jira/internal/shared/telemetry/telemetry.go`
- Authentication: none configured (endpoint-only, `OTEL_EXPORTER_OTLP_ENDPOINT`); falls back to a stdout/console exporter when unset

## API Integrations

### Toggl Track REST API v9 (outbound, `mcp`)

- Purpose: List time entries and (on demand) projects, to serve the `list_time_entries` MCP tool
- Location: `mcp/src/toggl/client.ts` (`TogglClient.listTimeEntries`, `TogglClient.listProjects`)
- Authentication: HTTP Basic Auth (API token as username, literal `api_token` as password)
- Key endpoints:
  - `GET /me/time_entries?start_date=&end_date=` — returns entries across every workspace on the account (not just the configured one); `mcp` filters to the configured/overridden workspace client-side after the fetch
  - `GET /me/projects?include_archived=false` — used only when a filtered time entry carries a `project_id`, and only on a cache miss/stale cache (see `docs/codebase/ARCHITECTURE.md`'s `mcp` — Overview)
- Rate limit: Toggl enforces 30 requests/hour on this API; `mcp` does not throttle, retry, or back off client-side — a `429` (with `Retry-After` when present) is passed straight back to the MCP client. A call costs at most 2 requests (1 with a warm project cache).
- Request timeout: 30s (`AbortSignal.timeout(30000)` in `TogglClient.request`)
- Error classification: any non-2xx or a network failure raises `TogglApiError`/`TogglNetworkError` (`mcp/src/toggl/client.ts`), mapped to a structured MCP error result by `mcp/src/errors.ts`

### Toggl Track Webhooks (inbound, `to-jira`)

- Purpose: Deliver `time_entry.created`, `time_entry.updated`, `time_entry.deleted` events (any other `request_type` is ignored with HTTP 200)
- Location: `to-jira/internal/webhook/handler.go` (`Handler.Receive`), `to-jira/internal/toggl/envelope.go` (`WebhookEnvelope`, `ParseEnvelope`)
- Authentication: HMAC-SHA256 over the raw request body, `X-Webhook-Signature-256: sha256=<hex>` header, verified before any parsing
- Key endpoint: `POST /webhooks/toggl` (the endpoint `to-jira` exposes to Toggl, not a Toggl API call)
- Response contract (HTTP status `to-jira` returns to Toggl, which controls Toggl's own retry behavior): 401 on bad/missing signature; 200 on validation failure, no-op, or successful write; 502 on a transient JIRA failure (triggers Toggl's at-least-once retry)

### JIRA Cloud REST API v3 (outbound)

- Purpose: Worklog CRUD against a specific issue, keyed by the JIRA issue key parsed from the Toggl entry's `[SLUG-NUMBER]` tag
- Location: `to-jira/internal/jira/worklog.go` (CRUD methods), `to-jira/internal/jira/client.go` (base `do` request builder)
- Authentication: HTTP Basic Auth (email + API token)
- Key endpoints:
  - `GET /rest/api/3/issue/{issueKey}/worklog` — paginated list, used to find an existing worklog by its `[TogglID:<id>]` ADF comment marker (JIRA has no server-side comment search, so this is always a full client-side scan)
  - `POST /rest/api/3/issue/{issueKey}/worklog` — create
  - `PUT /rest/api/3/issue/{issueKey}/worklog/{worklogId}` — update
  - `DELETE /rest/api/3/issue/{issueKey}/worklog/{worklogId}` — delete
- Error classification: `jira.classifyStatus` maps 2xx → nil, 5xx/429 → `*jira.TransientError`, any other non-2xx (400/404/etc.) → `*jira.PermanentError`. `sync.Processor` currently treats both the same way (502 to Toggl, `jira_api_errors_total` incremented) since a permanently-wrong request retries harmlessly with the same input.

## Webhooks

### Toggl Track (consumed)

- Purpose: Real-time time-entry change notifications
- Location: `to-jira/internal/webhook/handler.go`
- Direction: Consumed (inbound)
- Events: `time_entry.created`, `time_entry.updated` (both routed through the same idempotent upsert path, `sync.Processor.Process`), `time_entry.deleted` (`sync.Processor.ProcessDelete`)
- Known limitation: the `time_entry.deleted` payload's exact field shape is unverified against a live Toggl subscription — see `to-jira/docs/NOT_IMPLEMENT.md`. Code defends against two hypothesized shapes (description present vs. absent) but has not been confirmed against a real delivery.

## Background Jobs

None in either package. `to-jira` has no scheduler, cron, or queue — every action happens synchronously within the webhook request that triggered it. This is an explicit design decision (see AD-002 in `.specs/STATE.md`): Toggl's own at-least-once webhook retry is the sole failure-recovery mechanism, in place of a reconciliation job. `mcp` has no scheduler either — every tool call resolves synchronously within the same MCP request; the on-disk project cache is refreshed lazily (on a stale/missing read), not on a timer.
