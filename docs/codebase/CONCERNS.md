# Codebase Concerns

**Analysis Date:** 2026-08-22

## Tech Debt

**Stateless-by-design orphaned worklog on issue-tag edit:**
- Issue: If an already-synced Toggl entry's `[SLUG-NUM]` tag is edited to point at a *different* JIRA issue, the new event only points at the new issue — the old worklog on the previous issue is undiscoverable without a persisted TogglID → issue mapping, and is left behind.
- Files: `to-jira/internal/sync/process.go`, `to-jira/internal/toggl/parse.go`
- Why: Deliberate trade-off (AD-001, `.specs/STATE.md`) — a full mapping database wasn't justified for one narrow edge case.
- Impact: An orphaned, unattributed worklog accumulates on the old issue each time this happens.
- Fix approach: A minimal persistent TogglID → JIRA issue key mapping, nothing more (per `to-jira/docs/NOT_IMPLEMENT.md`). Deferred until observed to happen often enough to matter.

**No in-process retry/backoff for JIRA 429s:**
- Issue: JIRA rate-limit responses (429) are treated identically to any other transient failure — no `Retry-After`-aware backoff.
- Files: `to-jira/internal/jira/worklog.go` (`classifyStatus`), `to-jira/internal/sync/process.go` (`jiraError`)
- Why: Considered unlikely to matter at single-tenant, personal-scale volume; relies on Toggl's own webhook retry instead.
- Impact: A 429 costs one extra Toggl retry-delivery cycle rather than being retried in-process.
- Fix approach: A bounded retry honoring `Retry-After` before falling back to the Toggl-retry safety net. Revisit only if 429s show up in practice.

**No per-TogglID concurrency guard (TOCTOU on duplicate-create):**
- Issue: Two near-simultaneous webhook deliveries for the same TogglID can both list worklogs, both see "not found," and both create one — two worklogs for one Toggl entry.
- Files: `to-jira/internal/sync/process.go` (`Process`, `upsertCreate`)
- Why: Considered very low probability for a single person's Toggl account.
- Impact: A duplicate worklog would need manual removal from JIRA if it happens.
- Fix approach: A per-TogglID in-process mutex serializing the lookup-then-write sequence. Revisit only if actually observed.

## Known Bugs

None identified from the code and test review. The one behavior explicitly flagged as unverified (below) is a pre-deployment risk, not a confirmed bug.

## Security Considerations

**Secrets sourced from process environment / `.env`, no secret manager:**
- Risk: `TOGGL_WEBHOOK_SECRET`, `JIRA_EMAIL`, `JIRA_API_TOKEN` are read via `os.Getenv` (optionally loaded from a local `.env` via `godotenv`), with no vault/secret-manager integration.
- Files: `to-jira/internal/shared/config/config.go`
- Current mitigation: `.env` is gitignored (`to-jira/.gitignore`); `.env.sample` documents variable names only, never values.
- Recommendations: When deployed to the k3d cluster referenced in the spec, source these from Kubernetes Secrets rather than a mounted `.env` file — no infra manifests exist yet to confirm how this will be handled, since deployment/CI is explicitly out of scope for v1 (`.specs/features/to-jira/spec.md`).

**No allowlist on JIRA project keys parsed from Toggl descriptions:**
- Risk: Any string matching `^\[([A-Z][A-Z0-9]{1,9})-(\d+)\]` is used as a JIRA issue key with no validation against a known-project allowlist.
- Files: `to-jira/internal/toggl/parse.go`
- Current mitigation: JIRA's own API error (404/400) is the validation signal — an unknown/wrong slug simply fails the JIRA call and surfaces via `jira_api_errors_total`.
- Recommendations: Accepted by explicit spec decision (single-operator, single-JIRA-account scope) — not a gap to close, but worth knowing if this codebase is ever adapted for multi-tenant use.

## Fragile Areas

**ADF comment marker extraction assumes a fixed structure:**
- Files: `to-jira/internal/jira/adf.go` (`ExtractTogglID`)
- Why fragile: Only inspects the first text run of the first paragraph node. If a worklog comment is manually re-edited via the JIRA UI after creation, the ADF document's shape may no longer match, causing `FindWorklogByTogglID` to report "not found."
- Common failures: A manually-edited worklog comment silently stops being recognized, and the next sync creates a duplicate worklog alongside it (not data loss, but a duplicate).
- Safe-modification notes: Documented as an accepted simplification in `.specs/features/to-jira/design.md`'s Risks & Concerns table, not tracked in `NOT_IMPLEMENT.md` (low likelihood, low consequence).
- Test coverage: `internal/jira/adf_test.go` covers the round-trip and the no-marker/structurally-different case, but not a manually-UI-edited real-world ADF shape.

## Missing Critical Features

**`time_entry.deleted` payload shape unverified against a live Toggl subscription:**
- Problem: Toggl's docs don't publish a concrete example of the delete payload's fields, unlike `created`/`updated`. `NormalizeEntry`/`ProcessDelete` are built defensively against two hypothesized shapes (description present vs. absent), but neither has been confirmed against a real delivery — the service has never been deployed.
- Files: `to-jira/internal/toggl/normalize.go`, `to-jira/internal/sync/process_delete.go`, `to-jira/docs/NOT_IMPLEMENT.md`
- Current workaround: A payload matching the unverified "description absent" shape is logged as `unsupported_delete`, counted in `validation_errors_total`, answered with HTTP 200 (not retried). Manual JIRA worklog deletion is required until confirmed.
- Blocks: Confident behavior of the delete path in production.
- Rough effort: One manual verification step once deployed with a live Toggl subscription — fire a real test delete, inspect the payload, update code if reality differs from either hypothesis. Explicitly called out in `.specs/features/to-jira/design.md`'s Risks table as a required pre-deployment step, not a Tasks-phase task (requires live credentials this project doesn't have yet).

**No deployment/CI pipeline exists yet:**
- Problem: The service is implemented and tested but has no `Dockerfile`, Kubernetes manifests, or CI workflow in this repo yet, despite the spec targeting a laptop-hosted k3d cluster.
- Files: none present (absence noted at repo root and `to-jira/`)
- Blocks: Any real-world verification of the delete-payload-shape concern above, and the observability stack (OTel is wired but has no collector to send to).
- Rough effort: Out of scope for `to-jira`'s own spec (explicitly deferred); tracked as a separate concern, not this project's task.

## Test Coverage Gaps

**Manually-edited JIRA worklog comment shape:** `ExtractTogglID`'s narrow structural assumption (see Fragile Areas above) is not exercised against a real UI-edited ADF document — only synthetic "wrong shape" inputs. Priority: low (duplicate, not data-loss, on failure). Difficulty to test: would require a captured real ADF payload from JIRA's UI-edit path.

**Live `time_entry.deleted` payload:** No test exists (or can exist pre-deployment) against Toggl's actual delete payload — both test cases are synthetic hypotheses. Priority: high once deployment is planned. Difficulty to test: requires a live Toggl subscription and a reachable endpoint, both currently unavailable.
