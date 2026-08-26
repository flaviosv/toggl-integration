# to-jira Specification

## Problem Statement

Time tracked in Toggl is currently logged to JIRA by manually exporting a report and re-entering it through a Claude skill — slow, batch-only, and create-only (no way to fix or remove a mislogged entry without going back to JIRA by hand). `to-jira` replaces that manual step with a webhook-driven Go service: Toggl notifies it in real time on create/update/delete, and it keeps the matching JIRA worklog in sync automatically.

## Goals

- [ ] A Toggl time entry tagged `[SLUG-NUMBER] <description>` produces a matching JIRA worklog on `SLUG-NUMBER` with no manual step, within the same webhook delivery.
- [ ] Editing or deleting that Toggl entry updates or removes the corresponding JIRA worklog the same way.
- [ ] Malformed entries and downstream failures are visible via logs/OTel without silently corrupting JIRA data (no duplicate worklogs, no garbage partial entries).

## Out of Scope

| Feature | Reason |
| --- | --- |
| Persistence layer / database | Every operation re-derives its target from the current event's own data plus a live JIRA worklog list+filter on the one known issue — no cross-issue search is ever needed, so no store is justified. See `to-jira/docs/NOT_IMPLEMENT.md`. |
| Reconciliation / drift-detection job | Not deployed yet; deferred by explicit decision. Toggl's own at-least-once webhook retry is the only safety net for v1 (see Assumptions). |
| Always-on hosting | Service will run on a laptop-hosted k3d cluster; downtime windows are accepted for now. |
| CI pipeline | Not deployed yet; no pipeline needed until it is. |
| JIRA project-slug allowlist validation | JIRA's own API error is the validation signal for an unknown/wrong slug. |
| Multi-tenant support | Single Toggl workspace, single JIRA account for v1. |
| Async/queued webhook processing | Synchronous, in-process handling only. |
| Slack/email notifications | Structured logs + OTel only for error surfacing. |
| In-process JIRA rate-limit backoff | Relies on Toggl's own retry (see Assumptions) rather than an in-process retry loop. See `to-jira/docs/NOT_IMPLEMENT.md`. |
| Per-TogglID concurrency locking | Low-probability race, accepted. See `to-jira/docs/NOT_IMPLEMENT.md`. |

---

## Assumptions & Open Questions

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --- | --- | --- | --- |
| Toggl `time_entry.deleted` webhook payload contents | Attempt to derive the JIRA issue key from the delete payload the same way as create/update; if the payload lacks enough data, log it as an unsupported-delete case rather than build a persistence layer | Toggl's docs don't publish a concrete example of the delete payload shape — this must be verified empirically against a real subscription as the first implementation task, before the delete handler is finalized | y (plan agreed; empirical verification is a required first task, see Design) |
| HTTP ack semantics to Toggl | Return non-2xx when a JIRA write fails for a transient reason (network error, JIRA 5xx/429); return 200 for validation failures (not retryable) and successful writes/no-ops | Turns Toggl's own at-least-once webhook retry into a free safety net for transient failures, with zero extra code, consistent with "no reconciliation job" | y |
| Create/Update webhook handling | Unified into a single idempotent "sync" (upsert) operation: look up `[TogglID:x]` on the derived issue, update if found, create if not | Toggl webhooks are at-least-once with no ordering guarantee — this makes duplicate delivery and out-of-order `created`/`updated` events both non-issues for free | y |
| JIRA project key format | `^\[([A-Z][A-Z0-9]{1,9})-(\d+)\]\s*(.*)$` | Matches standard JIRA project key convention (2-10 chars, uppercase letters/digits, starting with a letter) | y |
| In-progress (still-running) Toggl entries | Skip syncing (no JIRA call, no error) when the entry has no stop time / a negative duration; rely on the later `updated` event fired when the timer stops | Toggl represents a running entry with a negative duration and no `stop` field. In practice, entries are always created with a static, pre-computed duration (the live start/stop tracker isn't used), so this should never trigger — kept as a near-zero-cost defensive guard against a nonsense write (garbage/negative time in JIRA) rather than a requirement addressing an active workflow. | y |
| JIRA API token expiry tracking | Config-only: an optional `JIRA_API_TOKEN_EXPIRES_AT` env var the operator sets manually (from the date Atlassian shows at token creation); service warns at startup within 14 days of it. No live introspection endpoint is used. | Could not confirm a JIRA REST API endpoint that returns a Basic-Auth API token's own expiry — flagged as uncertain per the knowledge verification chain rather than fabricated | y |

**Open questions:** none — all resolved or logged above.

---

## User Stories

### P1: Webhook authenticity ⭐ MVP

**User Story**: As the service operator, I want unauthenticated requests to the webhook endpoint rejected, so a random internet POST can't write time to my JIRA account.

**Why P1**: The endpoint will sit behind a public tunnel; without this it's an open write API to your JIRA.

**Acceptance Criteria**:

1. WHEN a request arrives at the webhook endpoint without a valid `X-Webhook-Signature-256` header (missing, malformed, or its HMAC-SHA256 of the raw body against the configured secret doesn't match) THEN system SHALL reject it with HTTP 401 and SHALL NOT parse or act on the payload.
2. WHEN the signature is valid THEN system SHALL proceed to event-type dispatch (`time_entry.created`, `time_entry.updated`, `time_entry.deleted`; any other event type is ignored with HTTP 200).

**Independent Test**: POST a payload with a bad/missing signature — expect 401, zero JIRA calls. POST the same payload with a correct signature — expect normal processing.

---

### P1: Sync a Toggl entry to a JIRA worklog ⭐ MVP

**User Story**: As someone who tracks time in Toggl, I want each `[SLUG-NUMBER]`-tagged entry to automatically appear as a JIRA worklog, so I never touch a manual export again.

**Why P1**: This is the core value of the project.

**Acceptance Criteria**:

1. WHEN a `time_entry.created` or `time_entry.updated` event passes signature verification THEN system SHALL parse the entry's description against `^\[([A-Z][A-Z0-9]{1,9})-(\d+)\]\s*(.*)$`.
2. WHEN the description does not match THEN system SHALL log a structured validation error (TogglID + raw description), increment `validation_errors_total`, make no JIRA call, and return HTTP 200.
3. WHEN the description matches but the entry is still running (no stop time / negative duration) THEN system SHALL skip syncing with no JIRA call and no error, and return HTTP 200.
4. WHEN the description matches and the entry is complete THEN system SHALL call `GET /rest/api/3/issue/{issueKey}/worklog` and search for a worklog whose ADF comment begins with `[TogglID:<id>]`.
5. WHEN no matching worklog is found THEN system SHALL create one via `POST /rest/api/3/issue/{issueKey}/worklog` with `timeSpentSeconds` and `started` from the Toggl entry, and comment `[TogglID:<id>] <text>` (text = description with the bracket tag stripped).
6. WHEN a matching worklog is found THEN system SHALL update it via `PUT .../worklog/{worklogId}` with the current duration/comment instead of creating a duplicate.
7. WHEN the create/update call succeeds THEN system SHALL increment `worklogs_created_total` or `worklogs_updated_total`, emit a trace span tagged with the TogglID, and return HTTP 200.
8. WHEN the create/update call fails for a transient reason (network error, JIRA 5xx, 429) THEN system SHALL log a structured error, increment `jira_api_errors_total`, and return a non-2xx status so Toggl retries redelivery later.
9. WHEN dry-run mode is enabled THEN system SHALL perform steps 1-6 (parse, validate, derive issue, look up existing worklog) but SHALL NOT call the create/update endpoint, instead logging the operation it would have performed, and return HTTP 200.

**Independent Test**: Send a `created` event with description `[ABC-123] Did the thing` — verify a worklog appears on `ABC-123` with comment `[TogglID:<id>] Did the thing`. Send an `updated` event for the same TogglID with a new duration — verify the same worklog is updated in place, not duplicated. Re-send the original `created` event (simulating Toggl's retry) — verify still no duplicate.

---

### P1: Remove a JIRA worklog when the Toggl entry is deleted ⭐ MVP

**User Story**: As someone who tracks time in Toggl, I want deleting an entry to remove its JIRA worklog, so JIRA reflects what I'm actually tracking.

**Why P1**: Listed as a core requirement alongside create/edit — this is bidirectional sync, not create-only.

**Acceptance Criteria**:

1. WHEN a `time_entry.deleted` event passes signature verification THEN system SHALL attempt to derive the JIRA issue key from the event payload using the same parsing as create/update.
2. WHEN the issue key can be derived THEN system SHALL call `GET .../worklog`, find the worklog whose comment begins with `[TogglID:<id>]`, and delete it via `DELETE .../worklog/{worklogId}` if found.
3. WHEN no matching worklog is found on the derived issue (already deleted, never synced, or previously failed validation) THEN system SHALL treat it as a no-op success, not an error, and return HTTP 200.
4. WHEN the delete payload does not contain enough data to derive the issue key THEN system SHALL log it as an unsupported-delete case, increment `validation_errors_total`, and return HTTP 200 — recorded as a known limitation in `to-jira/docs/NOT_IMPLEMENT.md`, not solved with a persistence layer.
5. WHEN the JIRA delete call fails for a transient reason THEN system SHALL log, increment `jira_api_errors_total`, and return non-2xx for Toggl retry, same as sync.
6. WHEN dry-run mode is enabled THEN system SHALL perform steps 1-2 (derive issue, look up worklog) but SHALL NOT call the delete endpoint, instead logging what it would have deleted, and return HTTP 200.

**Independent Test**: Send a `deleted` event for a previously-synced TogglID — verify the matching worklog is removed. Send a `deleted` event for an unknown TogglID — verify HTTP 200, no error, no side effects beyond the lookup.

---

### P3: JIRA API token expiry reminder

**User Story**: As the service operator, I want a heads-up before my JIRA API token expires, so this unsupervised service doesn't silently start failing every write.

**Why P3**: Operationally useful, not core sync functionality.

**Acceptance Criteria**:

1. WHEN `JIRA_API_TOKEN_EXPIRES_AT` is configured and the current date is within 14 days of it THEN system SHALL log a warning at startup.
2. WHEN `JIRA_API_TOKEN_EXPIRES_AT` is not configured THEN system SHALL log an informational note at startup that token expiry isn't being tracked (no error).

---

## Edge Cases

- WHEN a duplicate `created` or `updated` delivery arrives for a TogglID already synced THEN system SHALL update in place, not create a duplicate (covered by the unified sync/upsert operation).
- WHEN an `updated` event arrives before its `created` event was ever processed (Toggl gives no ordering guarantee) THEN system SHALL treat it identically to a `created` event via the same upsert lookup.
- WHEN a `deleted` event arrives for a TogglID with no matching worklog THEN system SHALL no-op with HTTP 200, not error.
- WHEN the description's bracket tag is followed by irregular whitespace or punctuation not covered by the regex THEN system SHALL treat it as a validation failure — no fuzzy matching.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| TJ-01 | P1: Webhook authenticity | Design | Pending |
| TJ-02 | P1: Sync — parse & validate format | Design | Pending |
| TJ-03 | P1: Sync — skip in-progress entries | Design | Pending |
| TJ-04 | P1: Sync — idempotent lookup by TogglID marker | Design | Pending |
| TJ-05 | P1: Sync — create when not found | Design | Pending |
| TJ-06 | P1: Sync — update in place when found | Design | Pending |
| TJ-07 | P1: Sync — transient failure → non-2xx for retry | Design | Pending |
| TJ-08 | P1: Sync — validation failure → log + 200 | Design | Pending |
| TJ-09 | P1: Sync — dry-run mode | Design | Pending |
| TJ-10 | P1: Delete — derive issue & delete matching worklog | Design | Pending |
| TJ-11 | P1: Delete — no-op when not found | Design | Pending |
| TJ-12 | P1: Delete — unsupported-payload known limitation | Design | Pending |
| TJ-13 | P1: Delete — dry-run mode | Design | Pending |
| TJ-14 | Cross-cutting: OTel traces + metrics | Design | Pending |
| TJ-15 | P3: JIRA token expiry reminder | Design | Pending |

**ID format:** `TJ-NN`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 15 total, 15 mapped to Design, 0 unmapped

---

## Success Criteria

- [ ] A `[SLUG-NUMBER]`-tagged Toggl entry produces a matching JIRA worklog with no manual step, within the same webhook delivery.
- [ ] Editing or deleting that entry updates or removes the JIRA worklog the same way.
- [ ] Zero duplicate worklogs under duplicate-delivery or out-of-order conditions.
- [ ] Malformed entries and downstream JIRA failures are visible in logs/OTel without operator action, and never silently corrupt JIRA data.
