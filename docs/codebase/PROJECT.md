# Project

## Overview

`toggl` is a personal-scale monorepo for services that automate the author's Toggl Track ↔ JIRA time-tracking workflow. Its first and currently only member is `to-jira`, a Go webhook service that keeps JIRA worklogs in sync with Toggl time entries in real time.

## Vision & Goals

Time tracked in Toggl was previously logged to JIRA by manually exporting a report and re-entering it through a Claude skill — slow, batch-only, and create-only (no way to fix or remove a mislogged entry without going back to JIRA by hand). `to-jira` replaces that manual step: Toggl notifies it in real time on create/update/delete, and it keeps the matching JIRA worklog in sync automatically.

Goals (from `.specs/features/to-jira/spec.md`):

- A Toggl time entry tagged `[SLUG-NUMBER] <description>` produces a matching JIRA worklog on `SLUG-NUMBER` with no manual step, within the same webhook delivery.
- Editing or deleting that Toggl entry updates or removes the corresponding JIRA worklog the same way.
- Malformed entries and downstream failures are visible via logs/OTel without silently corrupting JIRA data (no duplicate worklogs, no garbage partial entries).

## Target Users

A single operator: the author, tracking their own time in Toggl and logging it against their own JIRA account. The spec explicitly scopes this to single-tenant, single-workspace use.

## Scope

**In scope:** real-time, bidirectional (create/update/delete) sync of `[SLUG-NUMBER]`-tagged Toggl time entries to JIRA worklogs, with webhook authenticity verification, dry-run mode, and OpenTelemetry instrumentation.

**Out of scope** (see `.specs/features/to-jira/spec.md` and `to-jira/docs/NOT_IMPLEMENT.md`):

- Persistence layer / database — every operation re-derives its target from the current event plus a live JIRA worklog lookup.
- Reconciliation / drift-detection job — Toggl's own at-least-once webhook retry is the only safety net for v1.
- Always-on hosting — runs on a laptop-hosted k3d cluster; downtime windows are accepted.
- CI pipeline — not deployed yet.
- JIRA project-slug allowlist validation — JIRA's own API error is the validation signal.
- Multi-tenant support.
- Async/queued webhook processing.
- Slack/email notifications — structured logs + OTel only.
- In-process JIRA rate-limit backoff and per-TogglID concurrency locking — low-probability edge cases, accepted and documented rather than solved.

## Status

Pre-deployment. `to-jira` is implemented and tested but has never run against a live Toggl subscription — the delete-event payload shape is unverified in production (see `to-jira/docs/NOT_IMPLEMENT.md`) and is flagged as a required manual verification step once deployed. No CI pipeline or hosting is wired up yet. The monorepo currently contains only this one service; a sibling MCP-based project is referenced as planned future work in `.specs/STATE.md`.
