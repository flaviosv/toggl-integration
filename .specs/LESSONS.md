# LESSONS — auto-maintained by scripts/lessons.py

> Machine-owned. Do NOT hand-edit. Changes are overwritten on the next `lessons.py` write.
> Canonical state lives in `.specs/lessons.json`. Edit lessons only via the script.
> promote_threshold=2 distinct features · window_days=45 · quarantine_threshold=2

## Confirmed (load these at Specify/Design)

Corroborated across multiple features. Safe to apply as guidance.

_none_

## Candidates (under observation — do NOT load as guidance yet)

Seen once or not yet corroborated. Tracked, not trusted.

### L-001 — When a task implements a function required to run 'at startup' or on a lifecycle hook, verify the composition root (main.go/di) actually calls it — a passing unit test on the function alone does not prove it's wired into the running service.
- signal: `ac_gap` · recurrence: 1 feature(s) · scope: `wiring` · harmful: 0
- features: to-jira
- evidence: validation.md TJ-15 AC1/AC2 (wiring)
- last seen: 2026-08-22T21:37:36Z

### L-002 — When an acceptance criterion names a specific documentation file to record a known limitation in, treat that file update as part of the Done-when checklist, not an implicit side effect of the code change.
- signal: `ac_gap` · recurrence: 1 feature(s) · scope: `docs` · harmful: 0
- features: to-jira
- evidence: validation.md TJ-12/AC4 (docs)
- last seen: 2026-08-22T21:37:36Z

### L-003 — When an AC requires both a metric and a trace span for the same event, use a real in-memory span recorder in tests, not a no-op tracer, so the span half of the requirement is actually asserted, not just the metric half.
- signal: `spec_precision_gap` · recurrence: 1 feature(s) · scope: `telemetry` · harmful: 0
- features: to-jira
- evidence: validation.md TJ-14/AC7 span assertion (telemetry)
- last seen: 2026-08-22T21:37:36Z

## Quarantined (failed when applied — ignore)

A confirmed lesson that recurred alongside failure. Kept for the maintainer to review.

_none_
