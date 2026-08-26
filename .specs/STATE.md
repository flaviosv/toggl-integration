# STATE

## Decisions

### AD-001
- **Decision**: `to-jira` (and any future sibling project in this monorepo touching the same Toggl↔JIRA sync domain) is stateless — no database, no ORM, no migrations. Every operation re-derives its target JIRA issue from the current event's own data plus a live `GET .../worklog` read on that one issue, filtered client-side for a `[TogglID:x]` comment marker.
- **Reason**: JIRA's REST API has no server-side worklog-comment search, so the only alternative to statelessness is a full mapping database — unjustified for a single small always-derivable fact. See `to-jira/docs/NOT_IMPLEMENT.md` for the one accepted edge case (issue-reassignment on edit) this trades away.
- **Trade-off**: An entry whose `[SLUG-NUMBER]` tag is edited to point at a different JIRA issue leaves its old worklog orphaned (undiscoverable without state) — documented, not solved.
- **Scope**: `to-jira` project; any future package/service that reads or writes the same TogglID↔worklog relationship must conform or explicitly supersede this.
- **Date**: 2026-08-22
- **Status**: active

### AD-002
- **Decision**: Toggl's own at-least-once webhook retry is the sole failure-recovery mechanism. The service returns non-2xx to Toggl when a JIRA write fails for a transient reason (network error, JIRA 5xx/429), letting Toggl redeliver later; it returns 200 for validation failures (not retryable) and successful writes/no-ops. No in-process retry/backoff, no reconciliation job, no queue.
- **Reason**: Gets free retry-on-transient-failure with zero extra code, consistent with the explicit decision not to build a reconciliation job or run on always-on infrastructure for v1.
- **Trade-off**: A webhook that never reaches the service at all (e.g. the cluster is down) is lost silently and permanently — no v1 mechanism detects that gap. Accepted explicitly.
- **Scope**: `to-jira` project; any future webhook-driven service in this monorepo should default to this same pattern unless a stronger guarantee is specifically required.
- **Date**: 2026-08-22
- **Status**: active

### AD-003
- **Decision**: OpenTelemetry SDK is wired for both traces and metrics from v1, with the OTLP exporter target configurable and defaulting to a stdout/console exporter when unset. No collector or backend (Grafana/Tempo/Loki/Prometheus) is stood up as part of this project.
- **Reason**: Cheap to instrument now, expensive to reconstruct retroactively; standing up a collector is a separate local-env infra concern, not this project's scope.
- **Trade-off**: Traces/metrics are not visible anywhere yet beyond stdout until a collector exists.
- **Scope**: `to-jira` project; sets the default OTel bootstrap pattern (`internal/shared/telemetry`) for future services in this monorepo, since neither dinherim nor applyr have OTel actually wired up despite the transitive dependency.
- **Date**: 2026-08-22
- **Status**: active

## Handoff

- **Feature**: to-jira (`.specs/features/to-jira/`)
- **Phase / Task**: Tasks — complete, pending user confirmation on tool assignment before Execute
- **Completed**: Specify (spec.md confirmed), Design (design.md confirmed), Tasks (tasks.md written — 21 tasks, 5 phases, all validation checks passed)
- **In-progress**: none
- **Next step**: Confirm MCP/skill tool assignment per task, confirm sub-agent batching (21 tasks → offer batch workers per Critical Rules), then begin Execute at T1
- **Blockers**: none
- **Uncommitted files**: `to-jira/docs/NOT_IMPLEMENT.md`, `.specs/features/to-jira/spec.md`, `.specs/features/to-jira/design.md`, `.specs/features/to-jira/tasks.md`, `.specs/STATE.md` — nothing committed yet, repo has zero commits
- **Branch**: main
