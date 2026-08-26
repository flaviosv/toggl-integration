# Tech Stack

This is a polyglot monorepo — two independent packages, each with its own language and runtime.
`to-jira` (Go) is documented in full below; `mcp` (TypeScript) has its own Core/Key
Libraries/Commands/Environment subsections and its deeper package detail in `mcp/CLAUDE.md`.

## Core — `to-jira`

- Language: Go 1.26 (`to-jira/go.mod`)
- Framework: Gin v1.12.0 (HTTP)
- Runtime: compiled Go binary
- Package manager: Go modules (`go.mod` / `go.sum`)
- Minimum versions: Go >= 1.26 (module `go` directive)

## Core — `mcp`

- Language: TypeScript 5.9.3 (`mcp/package.json`, `mcp/tsconfig.json`)
- Framework: `@modelcontextprotocol/sdk` v1.30.0 (`McpServer`/`StdioServerTransport`)
- Runtime: Node.js, compiled to `dist/` via `tsc` (ES2022 target, `NodeNext` module resolution), run as a stdio child process
- Package manager: npm (`package.json` / `package-lock.json`)
- Minimum versions: Node.js >= 18 (relies on the built-in `fetch` client, per `mcp/README.md`)

## Key Libraries — `to-jira`

| Library | Version | Purpose | Modern Usage |
| ------- | ------- | ------- | ------------ |
| `github.com/gin-gonic/gin` | v1.12.0 | HTTP server/router | `gin.New()` + explicit middleware (`gin.Recovery()`), route groups for middleware chains |
| `github.com/go-playground/validator/v10` | v10.30.3 | Struct-tag config validation | `validator.New().Struct(&cfg)` against `validate:"required"` tags |
| `github.com/joho/godotenv` | v1.5.1 | `.env` file loading for local dev | `godotenv.Load()` before reading `os.Getenv` |
| `go.opentelemetry.io/otel` (+ `sdk`, `metric`, `trace`) | v1.45.0 | Tracing and metrics SDK | `TracerProvider`/`MeterProvider` via `otel.SetTracerProvider`/`SetMeterProvider`, `otel.Tracer(name)` for spans |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`, `otlpmetric/otlpmetrichttp` | v1.45.0 | OTLP-over-HTTP exporters | Used when `OTEL_EXPORTER_OTLP_ENDPOINT` is set |
| `go.opentelemetry.io/otel/exporters/stdout/{stdouttrace,stdoutmetric}` | v1.45.0 | Console exporters | Default when no OTLP endpoint is configured |

Stdlib only otherwise: `net/http`, `encoding/json`, `crypto/hmac`/`crypto/sha256`, `regexp`, `log/slog`, `context`, `testing`.

## Key Libraries — `mcp`

| Library | Version | Purpose | Modern Usage |
| ------- | ------- | ------- | ------------ |
| `@modelcontextprotocol/sdk` | ^1.30.0 | MCP protocol server (`McpServer`, `StdioServerTransport`, Zod-typed tool registration) | `server.registerTool(name, { description, inputSchema }, handler)` |
| `dotenv` | ^17.4.2 | Loads `mcp/.env` at process startup | `dotenv.config({ path, quiet: true })` before `loadConfig` reads `process.env` |
| `zod` | ^4.4.3 | Runtime input-schema validation for the MCP tool's arguments | `z.object({...}).refine(...)` for cross-field date-range rules |
| `typescript` (dev) | ^5.9.3 | Compiles `src/**/*.ts` to `dist/` | `strict: true`, `NodeNext` resolution |
| `openapi-typescript` / `swagger2openapi` (dev) | ^7.13.0 / ^7.0.8 | Regenerates `src/toggl/generated.ts` from the committed Toggl OpenAPI spec | `npm run generate:openapi` |

No HTTP client dependency — the built-in Node `fetch` is used directly (`mcp/src/toggl/client.ts`).

## Backend

- API Style — `to-jira`: REST (single inbound webhook endpoint), consuming JIRA Cloud REST API v3 as an outbound REST client.
- API Style — `mcp`: MCP tool over stdio (not HTTP), consuming Toggl Track REST API v9 as an outbound REST client.
- Database: none in either package — both are intentionally stateless. `to-jira`: see `docs/codebase/ARCHITECTURE.md` and AD-001 in `.specs/STATE.md`. `mcp`: a local disk file caches project names only (not a database) — see `mcp/CLAUDE.md`.
- Authentication — `to-jira`: inbound requests verified via HMAC-SHA256 webhook signature (`X-Webhook-Signature-256`); outbound JIRA calls use HTTP Basic Auth (email + API token).
- Authentication — `mcp`: outbound Toggl calls use HTTP Basic Auth (API token as username, literal `api_token` as password); no inbound auth surface (stdio, not network-exposed).

## Testing

- `to-jira` — Unit: stdlib `testing`, table-driven style, `httptest.Server`/`httptest.NewRecorder()` for HTTP-boundary tests, hand-written fakes for the `sync.JiraClient` interface (no mocking framework, no live network calls).
- `mcp` — Unit/Integration: Node's built-in `node --test` (no third-party test framework) against compiled `dist/**/*.test.js`; a shared local `http.Server` fake stands in for the Toggl API, `InMemoryTransport` for in-process MCP client/server tests, real subprocess spawn only for `index.test.ts`.

Detail in `docs/codebase/TESTING.md`.

## External Services

- Toggl Track: inbound webhook source for `to-jira` (`POST /webhooks/toggl`); outbound REST API v9 target for `mcp` (`GET /me/time_entries`, `GET /me/projects`). Detail in `docs/codebase/INTEGRATIONS.md`.
- JIRA Cloud REST API v3: outbound worklog CRUD target for `to-jira`. Detail in `docs/codebase/INTEGRATIONS.md`.
- OpenTelemetry OTLP collector (optional, no collector deployed yet, `to-jira` only): trace/metric export target when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

## Commands — `to-jira`

| Task | Command |
| ---- | ------- |
| Run all tests | `go test ./...` (run from `to-jira/`) |
| Run one package's tests | `go test ./internal/<pkg>/...` |
| Build | `go build ./...` |
| Vet | `go vet ./...` |
| Full gate check | `go build ./... && go vet ./... && go test ./...` |
| Run locally | `go run ./main.go` (from `to-jira/`, requires `.env` populated from `.env.sample`) |

No `Makefile`, lint tool, or CI pipeline exists yet — the project's spec explicitly defers CI (`.specs/features/to-jira/spec.md`'s Out of Scope table).

## Commands — `mcp`

| Task | Command |
| ---- | ------- |
| Install dependencies | `npm install` (run from `mcp/`) |
| Build | `npm run build` (`tsc -p tsconfig.json`) |
| Run all tests | `npm test` (builds, then `node --test 'dist/**/*.test.js'`) |
| Run fast unit tests only | `npm run test:unit` (`config` + `tools/**`) |
| Run integration test only | `npm run test:integration` (`index.test.ts`, real subprocess spawn) |
| Regenerate Toggl API types | `npm run generate:openapi` (after `openapi/toggl.swagger2.json` changes) |
| Run locally | `node dist/index.js` (requires `mcp/.env` populated from `.env.sample`; also how it's registered as an MCP server — see `mcp/README.md`) |

No lint tool or CI pipeline exists yet for `mcp` either.

## Local Development Setup

- **`to-jira`:** Copy `to-jira/.env.sample` to `to-jira/.env` and fill in the required values; `config.Load()` calls `godotenv.Load()` on startup, so `.env` (gitignored) is picked up automatically. No database, container, or external service needs to run locally. No mock/stub server for JIRA or Toggl; tests fake the `sync.JiraClient` interface or spin up `httptest.Server` per test. Default port: `8080` (`PORT` env var).
- **`mcp`:** Copy `mcp/.env.sample` to `mcp/.env` and fill in `TOGGL_API_TOKEN`/`TOGGL_WORKSPACE_ID`; `dotenv.config()` in `src/index.ts` loads it automatically at startup. No database or container needed. No mock Toggl server for manual local runs; tests use an in-process fake HTTP server (`src/tools/test-harness.ts`). Not network-exposed — runs only as a stdio child process of an MCP client.

## Environment Configuration

**`to-jira`:**

| Variable | Description |
| -------- | ----------- |
| `TOGGL_WEBHOOK_SECRET` | Shared secret used to verify the Toggl webhook HMAC-SHA256 signature. Required. |
| `JIRA_BASE_URL` | JIRA Cloud instance base URL; must start with `https://`. Required. |
| `JIRA_EMAIL` | JIRA account email for Basic Auth. Required. |
| `JIRA_API_TOKEN` | JIRA API token for Basic Auth. Required. |
| `JIRA_API_TOKEN_EXPIRES_AT` | Optional calendar date (`YYYY-MM-DD`) the operator copies from Atlassian's token-creation screen; service warns at startup within 14 days of it. |
| `DRY_RUN` | Optional boolean; when true, the sync pipeline runs through validation/lookup but skips the actual JIRA write. Defaults to `false`. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Optional OTLP HTTP endpoint; when unset, traces/metrics go to stdout instead. |
| `PORT` | Optional HTTP listen port. Defaults to `8080`. |

**`mcp`:**

| Variable | Description |
| -------- | ----------- |
| `TOGGL_API_TOKEN` | Toggl API token, used as the HTTP Basic Auth username against the Toggl API. Required. Never logged. |
| `TOGGL_WORKSPACE_ID` | Positive integer; the workspace entries are filtered to by default, overridable per call. Required. |
| `TOGGL_CACHE_PATH` | Where the local project-name cache file is written. Optional, defaults to `~/.cache/toggl-mcp/projects.json`. |

## Development Tools

- Editor/agent tooling: `tlc-spec-driven` spec-driven workflow (`.specs/features/to-jira/`, `.specs/features/TOGGL-2-time-entries-mcp/`) used to plan and implement both services.
