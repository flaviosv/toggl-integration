# Toggl Time Entries MCP

A stateless [Model Context Protocol](https://modelcontextprotocol.io/) server that exposes Toggl
Track's Time Entries API to an MCP-capable agent. It runs over stdio as a local child process, so
it's designed for a single personal Toggl account operated by one client at a time — not a shared
or multi-tenant service.

The server's one differentiator over calling the Toggl API directly is ticket-code project
matching: a time entry described as `[JSP-10] Creating form` resolves to the Toggl project whose
name ends in `JSP` automatically. If the code matches zero or more than one active project, the
server returns a structured error listing the candidates instead of guessing, so an entry never
lands under the wrong project silently.

Toggl enforces a tight **30 requests/hour** rate limit on this API. Every tool call here costs at
least one of those requests, so the server also caches the project list locally (7-day TTL) to
keep routine creates and updates from burning a request on every call.

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
| `TOGGL_WORKSPACE_ID` | Yes | — | Positive integer. The workspace all reads and writes are scoped to by default; overridable per call on tools that accept `workspace_id`. |
| `TOGGL_CACHE_PATH` | No | `~/.cache/toggl-mcp/projects.json` | Where the local project cache (used for ticket-code matching) is written. |

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
| `create_time_entry` | Create a Toggl time entry. Resolves a project from a `[TICKET-1]` prefix in the description unless `project_id` is supplied explicitly. |
| `delete_time_entry` | Delete a Toggl time entry by id. |
| `get_time_entry` | Get a single Toggl time entry by id, curated to `id`, `description`, `start`, `stop`, and `project`. |
| `list_time_entries` | List Toggl time entries within a date range, curated and filtered to a single workspace, in exactly one Toggl API call. |
| `refresh_projects` | Force a refetch of the Toggl project cache, bypassing the 7-day TTL. |
| `update_time_entry` | Update a Toggl time entry (read-modify-write). Re-resolves the project only when the description changes without an explicit `project_id`. |

## Rate limit

Toggl allows **30 requests/hour** on this API. The server does not throttle, retry, or back off
client-side — every tool call fires immediately. If Toggl returns `429`, the server passes it back
to the agent as-is, including the `Retry-After` header when present, so the agent (or you) can
decide whether to wait.

## Ticket-code project matching

When a `description` starts with a `[SLUG-NUMBER]` tag — for example `[JSP-10] Creating form` —
the server extracts `JSP` as the ticket code and looks for exactly one active Toggl project whose
name, after stripping a leading `[...]` bracket group, matches `JSP` case-insensitively. Matching
never falls back to substring or fuzzy comparison.

- **Exactly one match**: the entry is created or updated under that project automatically.
- **Zero or multiple matches**: the call fails with a structured error naming the extracted code
  and listing every candidate project's `id` and `name`. Nothing is created or updated.
- **No tag and no explicit `project_id`**: the entry is created or updated with no project
  attached, and the result reports `project: null`.
- **Explicit `project_id`**: bypasses code extraction and matching entirely, and is used verbatim.
