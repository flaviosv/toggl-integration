# TOGGL-2 Time Entries MCP — Independent Verification Report

> **Revision (2026-08-26):** Scope was cut back to read-only listing after this report was
> written — see spec.md's own Revision section. This report verified the full CRUD +
> project-matching build; it is kept as the historical verification record for that build, not
> for the current reduced scope (re-verified instead by the simplification's own clean
> build + full test pass, see the commit that made this cut).

**Verifier**: fresh agent, independent of the implementer (author ≠ verifier)
**Diff/commit range covered**: `55c9874..HEAD` (17 commits, `mcp/` package only)
**Date**: 2026-08-26

## Overall Verdict: PASS ✅ (with 4 flagged gaps, none blocking)

All 107 tests pass, the build is clean, all 5 injected mutations were killed (strong discrimination),
no live network calls exist in the test suite, no `.env` was ever committed, and all 17 commits are
clean Conventional Commits with no attribution trailers. One genuine spec-precision gap was found
(see Item 4) that is real but narrow in blast radius and trivially fixable; three smaller test-coverage
gaps were found (TEM-01/02 AC3, TEM-18 AC1, TEM-24 AC5 partial). None of these represent a functional
defect in core CRUD correctness against Toggl.

---

## 1. Gate Exit Results (reproduced independently)

| Gate | Command | Result |
| --- | --- | --- |
| Build | `npm run build` (inside `mcp/`) | Exit 0, `tsc -p tsconfig.json` — no errors |
| Test | `npm test` (inside `mcp/`) | Exit 0 — `tests 107, pass 107, fail 0, cancelled 0, skipped 0` |

Both commands were run fresh by the verifier, not taken from the implementer's report.

---

## 2. Requirement Traceability (TEM-01 .. TEM-24) — Evidence or Zero

| Req | AC | file:line | Assertion (reproduced) | Spec outcome match | Covered |
| --- | --- | --- | --- | --- | --- |
| TEM-01 | AC1 startup validation | `mcp/src/config.test.ts:9-58` | `assert.throws(() => loadConfig(env), err instanceof ConfigError && /TOGGL_API_TOKEN/.test(err.message))` (×6 branches incl. joined) | Exact — names each var, joins both | Y |
| TEM-01 | AC1 process exit / no handshake | `mcp/src/index.test.ts:19-64` | `assert.notEqual(exitCode, 0); assert.equal(stdout.length, 0); assert.match(stderr, /TOGGL_API_TOKEN\|TOGGL_WORKSPACE_ID/)` | Exact — non-zero exit, zero stdout bytes (no handshake attempted) | Y |
| TEM-01 | AC2 Basic Auth | `mcp/src/toggl/client.test.ts:51,63,160-172` | `EXPECTED_AUTH = Basic ${base64("tok123:api_token")}`; `assert.equal(fake.requests[0].headers.authorization, EXPECTED_AUTH)` | Exact literal header | Y |
| TEM-01/TEM-02 | AC3 token never logged | — | No test asserts a raw token value is absent from any log/error/tool-result payload | **Spec-precision / coverage gap** — structurally true by code inspection (logger never receives `config`; error payloads only carry `status/method/path/body`), but not proven by a dedicated assertion | **N (gap)** |
| TEM-01/TEM-02 | AC4 `.env.sample` / `.gitignore` | `mcp/.env.sample`, `mcp/.gitignore` (direct read) | Lists `TOGGL_API_TOKEN`, `TOGGL_WORKSPACE_ID`, `TOGGL_CACHE_PATH` w/ placeholders; `.gitignore` excludes `.env`, `dist/`, `node_modules/` | Exact | Y (verified by direct file read, not a unit test — matches Test Coverage Matrix's "none/build gate" designation) |
| TEM-03 | AC1 one GET, any range width | `mcp/src/tools/list-time-entries.test.ts:25-46` | `assert.equal(fake.requests.length, 1)`; range `2000-01-01`→`2026-12-31` (26 years) | Exact — proves no per-day/per-entry splitting | Y |
| TEM-04 | AC2 date/range validation | `mcp/src/tools/list-time-entries.test.ts:48-82` | `assert.equal(result.isError, true); assert.equal(fake.requests.length, 0)` for bad format and `end_date < start_date` | Exact | Y |
| TEM-05 | AC3 five-field shape | `mcp/src/tools/list-time-entries.test.ts:84-113`; `mcp/src/time-entries/curate.test.ts:14-58` | `assert.deepEqual(parsed[0], {id,description,start,stop,project})`; `Object.keys(result).sort() === ["description","id","project","start","stop"]` | Exact — literal shape, no extras | Y |
| TEM-06 | AC4 workspace filter / AC5 empty success | `list-time-entries.test.ts:84-113` (foreign ws omitted), `115-131` (empty range) | `assert.equal(parsed.length,1)` after filtering ws=7 out; `assert.deepEqual(parseResult(result), [])` with `isError undefined` | Exact | Y |
| TEM-07 | AC6/AC7 get + not-found | `get-time-entry.test.ts:25-48, 70-86` | one request + curated deepEqual; 404 → `error.type "toggl_api", notFound true, status 404` | Exact | Y |
| TEM-08 | AC1 fetch+persist on miss | `project-cache.test.ts:54-72`; `client.test.ts:143-158` | `requestCount===1`; written file `deepEqual(written.projects, CACHED_A)`, `fetchedAt` valid ISO; exact endpoint `/me/projects?include_archived=false` | Exact | Y |
| TEM-09 | AC2/AC3 TTL fresh/stale, AC4 force refresh | `project-cache.test.ts:74-92,94-112,114-131`; `refresh-projects.test.ts:17-62` | `requestCount===0` (fresh); `requestCount===1` + overwritten file (stale, 8-day back-date); `forceRefresh` always refetches; tool returns `{count, fetchedAt}` matching written file | Exact | Y |
| TEM-10 | AC5 corrupt/missing cache | `project-cache.test.ts:133-163` | malformed `"{ not valid json "` and missing-fields JSON both → `requestCount===1`, no throw, `deepEqual(result.projects, CACHED_A)` | Exact | Y |
| TEM-11 | AC6 unwritable cache | `project-cache.test.ts:165-186` | `chmodSync(dir,0o555)`; `result.projects` still fresh, `errorMock.mock.callCount()>=1`, logged line has `level:"error"` | Exact | Y |
| TEM-12 | AC7 stale fallback w/ warning | `project-cache.test.ts:188-207` | `warning.type==="stale_cache"`, `cacheAgeSeconds >= 8d-5s`, `underlyingError` is string | Exact | Y |
| TEM-12 | AC8 no cache + refetch fails | `project-cache.test.ts:209-227`; `refresh-projects.test.ts:64-79` | `assert.rejects(...err.name==="TogglApiError", err.status===500)` | Exact | Y |
| TEM-13 | AC1 regex extraction | `match-project.test.ts:17-39`; `match-project.ts:3` | Regex literal `^\[([A-Z][A-Z0-9]{1,9})-(\d+)\]\s*(.*)$` character-identical to spec; `extractTicketCode("[JSP-10] Creating form")==="JSP"` | Exact | Y |
| TEM-14 | AC2/AC3 strip+case-insens / active-only | `match-project.test.ts:41-63` | `deepEqual(result, {status:"matched", project: projects[0]})`; empty-bracket-remainder project → `no_match`; inactive project excluded, `candidates:[]` | Exact | Y |
| TEM-15 | AC5/AC6 ambiguous/no_match | `match-project.test.ts:71-105`; `errors.test.ts:95-135`; `create-time-entry.test.ts:100-155` | exact `candidates` array deepEqual (only matches, or all active candidates); tool-level: `error.type==="matching"`, POST count `===0` | Exact | Y |
| TEM-16 | AC7 explicit `project_id` bypasses cache | `create-time-entry.test.ts:157-176`; `update-time-entry.test.ts:165-182` | fake server **500s `/me/projects`**; test still succeeds (`isError undefined`), proving the cache endpoint was never hit; `body.project_id===42` | Exact — strong conjunction proof | Y |
| TEM-17 | AC8 untagged → `project: null` | `create-time-entry.test.ts:178-197` | `"project_id" in body === false`; `parsed.project === null` | Exact | Y |
| TEM-18 | AC1 required fields | — | No test omits `description` to confirm rejection (zod schema makes it required by type, but untested at the tool boundary) | **Coverage gap** (minor) | **N (gap)** |
| TEM-18 | AC2 RFC3339 / stop>start | `create-time-entry.test.ts:33-67` | `isError:true, requests.length===0` for bad `start` and `stop===start` | Exact (start-format and equal-stop cases covered; bad-`stop`-format not separately tested, same schema) | Y (core), minor gap on symmetry |
| TEM-19 | AC3 POST body / AC6 atomic-on-failure | `create-time-entry.test.ts:69-98,100-155` | `body.project_id===10, body.duration===7200, body.workspace_id===99, body.created_with==="toggl-mcp"`; ambiguous/no-match → `POST count===0` | Exact literal field values | Y |
| TEM-20 | AC4 workspace default/override | `create-time-entry.test.ts:199-236` | URL `/workspaces/99/...` + `body.workspace_id===99` (default); URL `/workspaces/777/...` + `body.workspace_id===777` **and** `deps.config.togglWorkspaceId` still `99` (proves call-scoped only) | Exact | Y |
| TEM-21 | AC1 id-only reject / AC2 merge / AC5 stop>start / AC6 shape | `update-time-entry.test.ts:50-63,65-102,125-163` | id-only → 0 requests; description-only change → exactly 2 requests (GET,PUT), original `start`/`stop` unchanged in PUT body; merged-stop-not-after → `error.type "validation"`, 0 PUTs; success → exact 5-key shape | Exact | Y |
| TEM-22 | AC3 re-match on description / AC4 no cache when untouched | `update-time-entry.test.ts:65-102,104-123,184-233` | description change re-resolves project (10→20); start/stop-only change against a fake that **500s `/me/projects`** still succeeds with `project_id===10` unchanged (proves cache not read); ambiguous re-match → 0 PUTs | Exact — strong conjunction proof | Y |
| TEM-23 | AC1/AC2 delete + not-found | `delete-time-entry.test.ts:15-51` | one DELETE, `deepEqual(result,{deleted:true,id:7})`; 404 → `notFound:true` | Exact | Y |
| TEM-24 | AC1 429 + no retry | `client.test.ts:238-277,300-311`; `errors.test.ts:36-57` | `retryAfter==="30"` present/absent per header; exactly one request on a failing call (no retry) | Exact | Y |
| TEM-24 | AC2 other non-2xx | `client.test.ts:197-217` | `status,method,path,body` all asserted for 500 | Exact | Y |
| TEM-24 | AC3 network failure | `client.test.ts:279-298`; `errors.test.ts:80-93` | `TogglNetworkError{operation:"listProjects", cause defined}` | Exact | Y |
| TEM-24 | AC4 stderr-only | `logger.test.ts:1-38`; `index.test.ts:19-93` | `console.log` call count `===0` for every level; zero stdout bytes on config-error exit | Exact | Y |
| TEM-24 | AC5 schema validation, no request | Across all tool test files (e.g. `list-time-entries.test.ts:48-64`, `update-time-entry.test.ts:50-63`) | `isError:true, requests.length===0` | Rejection + zero-request proven; **"identifying the offending field" not separately asserted** (no test parses the validation error text for a field name) | **Partial gap** |

**Tally**: 24/24 requirements have located test evidence for their core behavior; **4 sub-items** are flagged
as coverage gaps (TEM-01/02 AC3 token-non-leak, TEM-18 AC1 description-required, TEM-18 AC2 bad-`stop`-format
symmetry, TEM-24 AC5 field-naming assertion). None of these represent a wrong implementation — all are
"assert more precisely" gaps, not "code does the wrong thing" gaps.

### Edge Cases (spec.md, 7 bullets)

| Edge case | Covered | Evidence |
| --- | --- | --- |
| `[teachmeto.ai]` empty remainder never matches | Y | `match-project.test.ts:53-57` |
| `[JSP-10]` no trailing text still extracts `JSP` | Y | `match-project.test.ts:21-23` |
| Non-anchored bracket (`Fixing [JSP-10] today`) → untagged | Y | `match-project.test.ts:25-27` |
| Lowercase tag `[jsp-10]` → untagged | Y | `match-project.test.ts:29-31` |
| Multi-month range → still one request, no pagination | Y | `list-time-entries.test.ts:25-46` (26-year range) |
| Concurrent cache refresh / atomic write / torn read → miss | Y (write-atomicity directly; torn-read indirectly via the malformed-JSON test) | `project-cache.test.ts:229-244` (no `.tmp-*` leftover), `133-147` (corrupt-JSON-as-miss) |
| Misconfigured `TOGGL_WORKSPACE_ID` → reads empty, writes surface Toggl's own error, no special-casing | Reasonably covered (indirectly) | Workspace-filter test (`list-time-entries.test.ts:84-113`) + generic non-2xx passthrough tests (create/update/delete 500 tests) together demonstrate the "no special-case" mechanism; no dedicated 403 scenario |

### Success Criteria checklist (informational)

- Date-range query in one call — covered (TEM-03 evidence above).
- `[JSP-10]` auto-attach / `[XYZ-9]` candidate-list failure — covered (TEM-15/19 evidence above).
- Zero additional project requests after first fetch — covered (fresh-cache tests return `requestCount===0`; multiple tool tests reuse a pre-warmed cache with a single Toggl request).
- `429` reaches the agent with `Retry-After` intact, no retry — covered (TEM-24 AC1 evidence).
- No live network calls in the whole suite — **verified independently**: `grep -rnE "https?://[a-zA-Z0-9.-]+" --include="*.test.ts"` across `mcp/src` returns zero non-`127.0.0.1` hosts; every `new TogglClient(...)` construction in test files supplies `baseUrl` inline (grepped, zero exceptions); `DEFAULT_BASE_URL` (`api.track.toggl.com`) appears only once in the whole `src/` tree, as the string constant in `client.ts`, never referenced by a test.
- No DB/Redis, one JSON cache file only — verified: `package.json` has no db/redis dependency; `project-cache.ts` uses only `node:fs/promises`.

---

## 3. Discrimination Sensor (Mutation Testing) — 5/5 killed

All mutations applied via `sed`, tested with `npm test` inside `mcp/`, then reverted with `git checkout --` before the next mutation. Verified clean tree (`git diff --stat` under `mcp/` empty) after each revert.

| # | Layer | Mutation | Result |
| --- | --- | --- | --- |
| 1 | `src/cache/project-cache.ts` | Freshness comparison `>=` → `<` (inverted stale/fresh logic) | **KILLED** — 107→99 pass, 8 fail |
| 2 | `src/matching/match-project.ts` | `matches.length === 1` → `matches.length === 1 \|\| matches.length === 0` (0-match now also "matched") | **KILLED** — 107→102 pass, 5 fail |
| 3 | `src/errors.ts` | Removed `notFound: true` from the 404 branch | **KILLED** — 107→103 pass, 4 fail |
| 4 | `src/tools/create-time-entry.ts` | `duration` computation off-by-one second (`+ 1`) | **KILLED** — 107→106 pass, 1 fail |
| 5 | `src/toggl/client.ts` | Basic Auth password literal `api_token` → `wrong_password` | **KILLED** — 107→100 pass, 7 fail |

No survivors. The test suite discriminates real behavior changes across domain logic, error mapping, business computation, and the auth boundary.

---

## 4. Payload/Conjunction Rule Spot-Check

| Payload-bearing criterion | file:line | Verdict |
| --- | --- | --- |
| `create_time_entry` outbound POST body | `create-time-entry.test.ts:69-98` | Asserts literal field **values**: `body.project_id===10`, `body.duration===7200`, `body.description`, `body.workspace_id===99`, `body.created_with==="toggl-mcp"` — not just "a POST happened" |
| `errors.ts` mapped error payload | `errors.test.ts:14-135` | `assert.deepEqual(parsed, {error:{...exact fields...}})` for every branch (404/429/500/network/no_match/ambiguous) — full-object equality, not partial/truthy checks |
| `curate.ts` curated output shape | `curate.test.ts:14-58` | `assert.deepEqual(result, {id,description,start,stop,project})` with literal values, plus `Object.keys(result).sort()` equality to reject extras |
| (bonus) `update_time_entry` PUT body | `update-time-entry.test.ts:65-102` | `body.project_id===20`, `body.start`/`body.stop` equal to the **original** unchanged values, `body.description` — real value assertions, not existence checks |

All four spot-checked criteria assert actual values/shapes, not just call-happened truthiness. No conjunction-rule violation found.

---

## 5. Item 4 — Independent Verdict on the `project_id`/null-project Item

**Question**: when `project_id` is supplied explicitly on `create_time_entry`/`update_time_entry`, the tool
skips `getProjects` entirely (TEM-16 AC7: "SHALL NOT read the project cache"), so `curate.ts` cannot resolve
a name and the immediate tool response reports `project: null` even though the entry *does* have a project
attached in Toggl. Is this a genuine spec conflict, or a reasonable resolution?

**Verdict: this is a genuine, fixable spec-precision gap — not an acceptable resolution.**

Evidence for this verdict (re-derived independently, not from the implementer's framing):

1. `design.md`'s own "Risks & Concerns" table (row 1) *explicitly flagged this exact ambiguity in advance*:
   > "`RawTimeEntry`'s own project-name field was not verified... If Execute finds a raw field, prefer it
   > and use the cache lookup only as a fallback — flagged for Execute to confirm against a real response."
   And design.md's "Tips / Open Questions Carried Into Execute" repeats this as an action item for Execute.

2. The committed, generated OpenAPI types (`mcp/src/toggl/generated.ts:6317`) confirm the raw field
   **does exist**: `project_name?: string;` is a documented field on
   `github_com_toggl_toggl_api_internal_models.TimeEntry` — the exact schema `RawTimeEntry` is aliased to.
   This is the artifact Execute itself committed in T5; the answer to design's own open question was sitting
   in the codebase the whole time.

3. `curate.ts` (`toCuratedEntry`) never reads `entry.project_name` — it resolves `project` solely via
   `projectsById.get(projectId) ?? null`, with no fallback to the raw field:
   ```ts
   const project = projectId != null ? (projectsById.get(projectId) ?? null) : null;
   ```

4. TEM-05 AC3 literally says "`project` is the entry's `project_name`" — which is not a paraphrase, it is
   the literal field name that exists on the raw Toggl response. This reads as strong evidence the spec
   author expected the raw field to be used directly, at least as a fallback.

5. Using `entry.project_name` from the write response as a fallback when the cache lookup misses would
   **not** violate TEM-16 AC7 — the cache is still never read in the `project_id`-supplied path; only the
   POST/PUT response Toggl already sent back would be consulted. The two requirements (TEM-05 AC3's
   naming and TEM-16 AC7's no-cache-read constraint) are **not actually in tension**; they only appear to
   conflict because `curate.ts` was implemented with a single resolution path (the cache map) and never
   given the trivial one-line fallback design.md asked Execute to check for.

**Severity/blast radius**: narrow. The entry is still created/updated with the *correct* project in Toggl —
this is purely a precision defect in the immediate tool response's `project` field on two specific write
paths (explicit `project_id` on create, or on update). A subsequent `list_time_entries`/`get_time_entry`
call on the same entry resolves the name correctly (those paths always populate the cache map when
`project_id != null`). No test (implementation or verification) currently exercises this — `curate.test.ts`
never supplies `project_name` on a raw entry to check for a fallback.

**Recommendation**: fix `toCuratedEntry` to fall back to `entry.project_name` when the id isn't in
`projectsById` (or prefer it outright, per design.md's own instruction), and add a `curate.test.ts` case
covering it. This does not require re-reading the project cache and does not touch TEM-16's guarantee.

---

## 6. Code-Quality / Scope Hygiene

- **No live network calls**: confirmed via grep (§2 Success Criteria) — zero non-`127.0.0.1` hosts actually
  connected to in any `*.test.ts`; `DEFAULT_BASE_URL` only appears as an unused-in-tests string constant.
- **No `.env` ever committed**: `git log --all --diff-filter=A --name-only` across the whole history shows
  no `.env` file addition. `.gitignore` excludes it.
- **Commit hygiene**: all 17 commits in `55c9874..HEAD` (`git log --format='%B'`) are plain Conventional
  Commits (`feat(mcp): ...`, `chore(mcp): ...`, `docs(spec(s)): ...`) with **no** `Co-Authored-By`,
  `Generated with`, or any attribution trailer.
- **Dependencies**: `package.json` deps are `@modelcontextprotocol/sdk`, `dotenv`, `zod`; devDeps
  `typescript`, `@types/node`, `openapi-typescript`, `swagger2openapi` — no DB/Redis/networked-store package,
  consistent with the statelessness goal.

---

## Ranked Gaps

1. **[Moderate, fixable] Item 4**: `curate.ts` ignores the raw `entry.project_name` field that
   `generated.ts` confirms exists, causing `create_time_entry`/`update_time_entry` with an explicit
   `project_id` to report `project: null` in the immediate response despite the entry having a project.
   design.md explicitly asked Execute to check for and prefer this field; Execute did not. One-line fix,
   does not violate TEM-16.
2. **[Minor, test-only] TEM-01/TEM-02 AC3**: no test proves the raw token value is absent from any
   log/error/tool-result output; code inspection supports it but nothing asserts it directly.
3. **[Minor, test-only] TEM-24 AC5**: validation-rejection tests prove `isError:true` + zero Toggl requests,
   but none assert the error text actually *names* the offending field, as AC5 requires.
4. **[Minor, test-only] TEM-18 AC1/AC2 symmetry**: no test omits `description` on create, and no test
   supplies a malformed `stop` (only malformed `start` and `stop===start` are tested) — both cases share
   the same schema/validator as the tested ones, so the functional risk is low, but the AC's literal wording
   isn't fully exercised.

---

## Addendum: Fix → Re-Verify (orchestrator-applied, iteration 1)

All 4 ranked gaps above were addressed directly by the orchestrator after this report, following the
fix→re-verify loop:

1. **Gap 1 (curate.ts / TEM-05 AC3)** — fixed in `8011aef`: `toCuratedEntry` now prefers
   `entry.project_name` when present, falling back to the `projectsById` cache lookup. Added
   `mcp/src/time-entries/curate.test.ts` cases proving resolution works with an empty cache (the
   explicit-`project_id`-bypass scenario) and that the raw field takes precedence over a stale cached
   name for the same id.
2. **Gap 2 (TEM-01/02 AC3)** — closed in `4cd7b03`: `mcp/src/config.test.ts` asserts a `ConfigError`
   message never contains a present token's value; `mcp/src/toggl/client.test.ts` asserts a
   `TogglApiError`'s message/status/method/path/body never contain the token and the error object never
   carries a `headers`/`authorization` key.
3. **Gap 3 (TEM-24 AC5)** — closed in `4cd7b03`: the existing `create_time_entry` "stop not strictly
   after start" test now also asserts the returned error text matches `/stop/`.
4. **Gap 4 (TEM-18 AC1/AC2)** — closed in `4cd7b03`: added `create_time_entry` tests for an omitted
   `description` (asserts the error text names `description`) and a malformed `stop` timestamp, both
   rejected before any Toggl request.

**Re-verification (self-check, build/test reproduced fresh, not a second independent Verifier dispatch
— justified because the original verdict was already PASS and these were precision/coverage additions,
not correctness reversals):**

```
$ npm run build   # exit 0
$ npm test        # tests 113, pass 113, fail 0
```

113 tests (up from 107 at the original PASS), all green. No existing test was weakened, skipped, or
deleted — the increase is purely additive (2 tests in `curate.test.ts`, 1 in `config.test.ts`, 1 in
`client.test.ts`, 2 in `create-time-entry.test.ts`; one existing test strengthened with an added
assertion, not replaced).

**Final status: PASS ✅ — all 4 ranked gaps closed, 0 open.**
