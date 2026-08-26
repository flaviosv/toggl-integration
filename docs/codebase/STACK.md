# Tech Stack

## Core

- Language: Go 1.26 (`to-jira/go.mod`)
- Framework: Gin v1.12.0 (HTTP)
- Runtime: compiled Go binary
- Package manager: Go modules (`go.mod` / `go.sum`)
- Minimum versions: Go >= 1.26 (module `go` directive)

## Key Libraries

| Library | Version | Purpose | Modern Usage |
| ------- | ------- | ------- | ------------ |
| `github.com/gin-gonic/gin` | v1.12.0 | HTTP server/router | `gin.New()` + explicit middleware (`gin.Recovery()`), route groups for middleware chains |
| `github.com/go-playground/validator/v10` | v10.30.3 | Struct-tag config validation | `validator.New().Struct(&cfg)` against `validate:"required"` tags |
| `github.com/joho/godotenv` | v1.5.1 | `.env` file loading for local dev | `godotenv.Load()` before reading `os.Getenv` |
| `go.opentelemetry.io/otel` (+ `sdk`, `metric`, `trace`) | v1.45.0 | Tracing and metrics SDK | `TracerProvider`/`MeterProvider` via `otel.SetTracerProvider`/`SetMeterProvider`, `otel.Tracer(name)` for spans |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`, `otlpmetric/otlpmetrichttp` | v1.45.0 | OTLP-over-HTTP exporters | Used when `OTEL_EXPORTER_OTLP_ENDPOINT` is set |
| `go.opentelemetry.io/otel/exporters/stdout/{stdouttrace,stdoutmetric}` | v1.45.0 | Console exporters | Default when no OTLP endpoint is configured |

Stdlib only otherwise: `net/http`, `encoding/json`, `crypto/hmac`/`crypto/sha256`, `regexp`, `log/slog`, `context`, `testing`.

## Backend

- API Style: REST (single inbound webhook endpoint), consuming JIRA Cloud REST API v3 as an outbound REST client.
- Database: none — the service is intentionally stateless (see `docs/codebase/ARCHITECTURE.md` and AD-001 in `.specs/STATE.md`).
- Authentication: inbound requests are verified via HMAC-SHA256 webhook signature (`X-Webhook-Signature-256`), not user auth. Outbound JIRA calls use HTTP Basic Auth (email + API token).

## Testing

- Unit: stdlib `testing`, table-driven style, `httptest.Server`/`httptest.NewRecorder()` for HTTP-boundary tests, hand-written fakes for the `sync.JiraClient` interface (no mocking framework, no live network calls). Detail in `docs/codebase/TESTING.md`.

## External Services

- Toggl Track: inbound webhook source (`POST /webhooks/toggl`). Detail in `docs/codebase/INTEGRATIONS.md`.
- JIRA Cloud REST API v3: outbound worklog CRUD target. Detail in `docs/codebase/INTEGRATIONS.md`.
- OpenTelemetry OTLP collector (optional, no collector deployed yet): trace/metric export target when `OTEL_EXPORTER_OTLP_ENDPOINT` is set.

## Commands

| Task | Command |
| ---- | ------- |
| Run all tests | `go test ./...` (run from `to-jira/`) |
| Run one package's tests | `go test ./internal/<pkg>/...` |
| Build | `go build ./...` |
| Vet | `go vet ./...` |
| Full gate check | `go build ./... && go vet ./... && go test ./...` |
| Run locally | `go run ./main.go` (from `to-jira/`, requires `.env` populated from `.env.sample`) |

No `Makefile`, lint tool, or CI pipeline exists yet — the project's spec explicitly defers CI (`.specs/features/to-jira/spec.md`'s Out of Scope table).

## Local Development Setup

- Copy `to-jira/.env.sample` to `to-jira/.env` and fill in the required values; `config.Load()` calls `godotenv.Load()` on startup, so `.env` (gitignored) is picked up automatically.
- No database, container, or external service needs to run locally — the service's own process is the only thing to start (`go run ./main.go`).
- No mock/stub server is provided for JIRA or Toggl in local dev; tests instead fake the `sync.JiraClient` interface or spin up `httptest.Server` per test.
- Default port: `8080` (`PORT` env var).

## Environment Configuration

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

## Development Tools

- Editor/agent tooling: `tlc-spec-driven` spec-driven workflow (`.specs/features/to-jira/`) used to plan and implement this service.
