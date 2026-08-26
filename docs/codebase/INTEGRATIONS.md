# External Integrations

## Integrations

**Toggl Track:**

- Type: Third-party time-tracking API (webhook source)
- Purpose: Fires real-time events when a time entry is created, updated, or deleted, driving the entire sync pipeline
- Data flow: Inbound only
- Protocol: Webhooks (HTTPS POST, JSON body, HMAC-SHA256 signed)
- Location: `to-jira/internal/webhook/handler.go` (entrypoint), `to-jira/internal/toggl/` (payload types/parsing)
- Authentication: Shared-secret HMAC-SHA256 signature verification (`TOGGL_WEBHOOK_SECRET`, `X-Webhook-Signature-256` header) — Toggl authenticates itself to `to-jira`; `to-jira` makes no outbound calls to Toggl

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

### Toggl Track Webhooks (inbound)

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

None. The service has no scheduler, cron, or queue — every action happens synchronously within the webhook request that triggered it. This is an explicit design decision (see AD-002 in `.specs/STATE.md`): Toggl's own at-least-once webhook retry is the sole failure-recovery mechanism, in place of a reconciliation job.
