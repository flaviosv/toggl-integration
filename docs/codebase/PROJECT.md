# Project

## Overview

`toggl` is a personal-scale monorepo for services and tools built around the author's Toggl Track usage. It has two members: `to-jira`, a Go webhook service that keeps JIRA worklogs in sync with Toggl time entries in real time, and `mcp`, a TypeScript [Model Context Protocol](https://modelcontextprotocol.io/) server that gives an MCP client (Claude Desktop, Claude Code) read-only access to the author's Toggl time entries.

## Vision & Goals

**`to-jira`:** Time tracked in Toggl was previously logged to JIRA by manually exporting a report and re-entering it through a Claude skill — slow, batch-only, and create-only (no way to fix or remove a mislogged entry without going back to JIRA by hand). `to-jira` replaces that manual step: Toggl notifies it in real time on create/update/delete, and it keeps the matching JIRA worklog in sync automatically.

Goals (from `.specs/features/to-jira/spec.md`):

- A Toggl time entry tagged `[SLUG-NUMBER] <description>` produces a matching JIRA worklog on `SLUG-NUMBER` with no manual step, within the same webhook delivery.
- Editing or deleting that Toggl entry updates or removes the corresponding JIRA worklog the same way.
- Malformed entries and downstream failures are visible via logs/OTel without silently corrupting JIRA data (no duplicate worklogs, no garbage partial entries).

**`mcp`:** Answering "what did I work on" questions in an MCP-capable client (Claude Desktop, Claude Code) previously meant leaving the conversation to check the Toggl web app. `mcp` (package name `toggl-mcp`) exposes the author's own Toggl time entries directly as an MCP tool, so an agent can list and reason about them in-conversation. It is deliberately read-only — never a replacement for `to-jira`'s write path, and unrelated to it in code.

Goals (from `.specs/features/TOGGL-2-time-entries-mcp/spec.md`):

- List Toggl time entries in a given date range, curated to `id`, `description`, `start`, `stop`, and `project`, filtered to one workspace.
- Stay well under Toggl's 30 requests/hour cap via a capped per-call request cost and a local project-name cache.
- Fail loudly and specifically on misconfiguration (every missing/invalid env var named at once) rather than a generic startup error.

## Target Users

A single operator: the author, tracking their own time in Toggl — logging it against their own JIRA account via `to-jira`, and querying it from their own MCP client via `mcp`. Both packages explicitly scope to single-tenant, single-workspace, single-operator use; `mcp` additionally assumes one MCP client connected at a time (one stdio child process per session).

## Scope

**In scope — `to-jira`:** real-time, bidirectional (create/update/delete) sync of `[SLUG-NUMBER]`-tagged Toggl time entries to JIRA worklogs, with webhook authenticity verification, dry-run mode, and OpenTelemetry instrumentation.

**Out of scope — `to-jira`** (see `.specs/features/to-jira/spec.md` and `to-jira/docs/NOT_IMPLEMENT.md`):

- Persistence layer / database — every operation re-derives its target from the current event plus a live JIRA worklog lookup.
- Reconciliation / drift-detection job — Toggl's own at-least-once webhook retry is the only safety net for v1.
- Always-on hosting — runs on a laptop-hosted k3d cluster; downtime windows are accepted.
- CI pipeline — not deployed yet.
- JIRA project-slug allowlist validation — JIRA's own API error is the validation signal.
- Multi-tenant support.
- Async/queued webhook processing.
- Slack/email notifications — structured logs + OTel only.
- In-process JIRA rate-limit backoff and per-TogglID concurrency locking — low-probability edge cases, accepted and documented rather than solved.

**In scope — `mcp`:** read-only `list_time_entries` MCP tool over stdio, local disk cache for project names (7-day TTL, stale-cache fallback), `.env`-based configuration.

**Out of scope — `mcp`** (see `.specs/features/TOGGL-2-time-entries-mcp/spec.md`): create/update/delete/get-single-entry tools, ticket-code-to-project matching (all scaffolded, then deliberately removed mid-build — commit `697a9d9`); client-side retry/backoff/throttling against Toggl's rate limit; multi-tenant or concurrent-MCP-client support; a bulk-delete tool (dropped pre-implementation — Toggl has no true bulk-delete endpoint).

## Status

**`to-jira`:** Pre-deployment. Implemented and tested but has never run against a live Toggl subscription — the delete-event payload shape is unverified in production (see `to-jira/docs/NOT_IMPLEMENT.md`) and is flagged as a required manual verification step once deployed. No CI pipeline or hosting is wired up yet.

**`mcp`:** Implemented, tested (83/83 tests passing), and code-reviewed — a complete-review + fix-review pass resolved 64 of 65 findings, with one (subprocess-exit verification in a test, `I5`) deliberately left open pending a follow-up decision (see `docs/codebase/CONCERNS.md`). Not yet published/distributed beyond local `claude mcp add`/Claude Desktop config; no packaging or versioning process exists beyond the `0.1.0` in `mcp/package.json`.
