# Not Implemented (Known Limitations)

Edge cases and limitations deliberately deferred from v1, kept here instead of solved so they aren't forgotten or silently rediscovered later. Each entry states the workaround for now and what would actually be needed to close it.

## Orphaned worklog when a Toggl entry's issue tag is changed

**Scenario**: An already-synced Toggl entry is edited so its `[SLUG-NUM]` tag changes to point to a *different* JIRA issue (e.g. correcting a mis-tagged entry from `PROJ-100` to `PROJ-200`).

**Why it's not handled**: to-jira is stateless by design — it never persists a TogglID → JIRA issue mapping. On every event it re-derives the target issue from the entry's *current* description, then locates the matching worklog by listing that issue's worklogs and matching the `[TogglID:<id>]` marker in the comment. When the tag itself changes, the new event only points at the new issue — nothing says where the *old* worklog lives, so it can't be found or cleaned up.

**Actual impact**: A new worklog gets created on the new issue. The old worklog is left behind, unattributed, on the old issue.

**Workaround for now**: Manually delete the stale worklog from the old issue if you notice it.

**What would close this**: A minimal persistent TogglID → JIRA issue key mapping — nothing more. The worklog itself is always re-derivable by listing the known issue's worklogs and filtering on the TogglID marker, so this never needs to grow into a full schema. Deferred because a database dependency wasn't justified for one narrow edge case; revisit only if it turns out to happen often enough to be annoying.

## JIRA rate limiting (429) has no in-process retry/backoff

**Scenario**: JIRA's per-issue write limits (20 writes/2s, 100 writes/30s) or global burst limits (50-100 req/s) are exceeded.

**Why it's not handled**: No `Retry-After`-aware backoff is implemented in-process. Considered unlikely to matter at single-tenant, personal-scale volume.

**Actual impact**: A JIRA write hit by a 429 is treated the same as any other transient JIRA API failure — logged, counted in `jira_api_errors_total`, and answered with a non-2xx response to Toggl so Toggl's own webhook retry (already at-least-once by design) redelivers the event later. This is a reasonable fallback, not a real gap, but is noted here since it was a deliberate choice not to add extra in-process retry logic on top of it.

**What would close this**: An in-process bounded retry honoring the `Retry-After` header before falling back to the Toggl-retry safety net. Revisit only if 429s show up in practice.

## Concurrent duplicate-create race (TOCTOU)

**Scenario**: Two webhook deliveries for the same TogglID are processed by the Go service in parallel (e.g. a genuine duplicate delivery arriving as a near-simultaneous second request). Both goroutines list the target issue's worklogs, both see "not found," and both create a worklog — resulting in two worklogs for one Toggl entry.

**Why it's not handled**: No per-TogglID locking/serialization is implemented. Considered very low probability for a single person's Toggl account, where duplicate deliveries are rare and even rarer to land within the same processing window.

**Actual impact**: In the unlikely event this happens, a duplicate worklog would need to be manually removed from JIRA.

**What would close this**: A per-TogglID in-process mutex (or lock) serializing the lookup-then-write sequence — no persistence required, just request-level serialization. Revisit only if this is actually observed.
