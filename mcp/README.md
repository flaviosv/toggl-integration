# Toggl Time Entries MCP

A stateless [Model Context Protocol](https://modelcontextprotocol.io/) server that exposes
read-only access to Toggl Track's time entries. It runs over stdio as a local child process, so
it's designed for a single personal Toggl account operated by one client at a time — not a shared
or multi-tenant service.

It has one job: list time entries within a date range, curated to `id`, `description`, `start`,
`stop`, and `project`. There is no create, update, or delete — this server never writes to Toggl.

Toggl enforces a tight **30 requests/hour** rate limit on this API. `list_time_entries` costs at
most two of those requests per call (one for the entries, one for the project list on a cold
cache) — the project list is cached locally on disk with a 7-day TTL to keep repeat calls to one
request.

## Prerequisites

- **Node.js 18 or later** (the server uses the built-in `fetch` client and ESM `NodeNext` module
  resolution — no HTTP client dependency).
- **A Toggl API token.** In Toggl Track, go to **Profile Settings** and copy the value under
  **API Token**.
- **Your Toggl workspace ID.** It's the numeric ID in the workspace URL when you have that
  workspace open in the Toggl Track web app, or under your workspace's settings page.

## Setup

1. Install dependencies:

   ```bash
   cd mcp
   npm install
   ```

2. Build the TypeScript sources to `dist/`:

   ```bash
   npm run build
   ```

3. Create your local `.env` from the committed sample and fill in your token and workspace ID:

   ```bash
   cp .env.sample .env
   ```

   Never commit `.env` — it's already listed in `mcp/.gitignore`.

`.env.sample` documents every variable the server reads:

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `TOGGL_API_TOKEN` | Yes | — | Your Toggl API token. Used as the username in HTTP Basic Auth against the Toggl API (the password is the literal string `api_token`). Never logged or echoed in any tool result. |
| `TOGGL_WORKSPACE_ID` | Yes | — | Positive integer. The workspace entries are filtered to by default; overridable per call via `workspace_id`. |
| `TOGGL_CACHE_PATH` | No | `~/.cache/toggl-mcp/projects.json` | Where the local project-name cache is written. |

If `TOGGL_API_TOKEN` or `TOGGL_WORKSPACE_ID` is missing or invalid, the server exits non-zero at
startup, naming every offending variable, before it registers a single tool.

## Configuring in Claude Desktop

Add an entry under `mcpServers` in your `claude_desktop_config.json`, pointing `command`/`args` at
the built `dist/index.js`. Replace `/absolute/path/to/toggl/mcp` with the actual path to this
package on your machine:

```json
{
  "mcpServers": {
    "toggl": {
      "command": "node",
      "args": ["/absolute/path/to/toggl/mcp/dist/index.js"],
      "env": {
        "TOGGL_API_TOKEN": "your_toggl_api_token",
        "TOGGL_WORKSPACE_ID": "1234567"
      }
    }
  }
}
```

Add `"TOGGL_CACHE_PATH"` to `env` only if you want to override the default cache location. Restart
Claude Desktop after editing the file.

## Configuring in Claude Code

Register the server with `claude mcp add`, again pointing at the built `dist/index.js` with your
own absolute path. Everything after `--` is passed to the server untouched, and `--env` accepts
one `KEY=value` pair per flag:

```bash
claude mcp add \
  --env TOGGL_API_TOKEN=your_toggl_api_token \
  --env TOGGL_WORKSPACE_ID=1234567 \
  toggl -- node /absolute/path/to/toggl/mcp/dist/index.js
```

Verify it registered with `claude mcp list`. By default this writes to local scope for the current
project; add `--scope user` to make it available across all your projects instead.

## Available tools

| Tool | Description |
| --- | --- |
| `list_time_entries` | List Toggl time entries within a date range (`start_date`, `end_date`), curated and filtered to a single workspace. |

## Rate limit

Toggl allows **30 requests/hour** on this API. The server does not throttle, retry, or back off
client-side — every tool call fires immediately. If Toggl returns `429`, the server passes it back
to the agent as-is, including the `Retry-After` header when present, so the agent (or you) can
decide whether to wait.

Note: `GET /me/time_entries` returns entries across every workspace the account belongs to
(not just the configured one), so the first request's payload size scales with all-workspace
entry volume; non-matching entries are filtered out client-side after the fact.
