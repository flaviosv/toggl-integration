import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { registerGetTimeEntry } from "./get-time-entry.js";
import { TogglClient } from "../toggl/client.js";
import { loadConfig } from "../config.js";
import {
  connectToolClient,
  makeTmpCachePath,
  parseResult,
  sendJson,
  startFakeToggl,
  writeProjectCache,
  type FakeTogglServer,
} from "./test-harness.js";
import type { ToolDeps } from "./schemas.js";

function makeDeps(fake: FakeTogglServer, cachePath: string): ToolDeps {
  const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
  const config = loadConfig({ TOGGL_API_TOKEN: "tok", TOGGL_WORKSPACE_ID: "99" });
  return { client, cachePath, config };
}

test("get_time_entry issues exactly one Toggl request and returns the curated shape (no project)", async () => {
  const fake = await startFakeToggl((_req, res) => {
    sendJson(res, 200, { id: 42, description: "entry", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", project_id: null });
  });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerGetTimeEntry], deps);
  try {
    const result = await client.callTool({ name: "get_time_entry", arguments: { id: 42 } });
    assert.equal(result.isError, undefined);
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].url, "/me/time_entries/42");
    assert.deepEqual(parseResult(result), {
      id: 42,
      description: "entry",
      start: "2026-01-01T00:00:00Z",
      stop: "2026-01-01T01:00:00Z",
      project: null,
    });
  } finally {
    await close();
    await fake.close();
  }
});

test("get_time_entry resolves a project name via a pre-warmed cache with exactly one Toggl request", async () => {
  const fake = await startFakeToggl((_req, res) => {
    sendJson(res, 200, { id: 7, description: "[TOGGL-3] work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T02:00:00Z", project_id: 3 });
  });
  const cachePath = makeTmpCachePath();
  writeProjectCache(cachePath, [{ id: 3, name: "[TOGGL-3] Gamma", active: true, workspaceId: 99 }]);
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerGetTimeEntry], deps);
  try {
    const result = await client.callTool({ name: "get_time_entry", arguments: { id: 7 } });
    assert.equal(fake.requests.length, 1);
    const parsed = parseResult(result) as { project: string | null };
    assert.equal(parsed.project, "[TOGGL-3] Gamma");
  } finally {
    await close();
    await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("a 404 returns a structured not-found error, not an empty success", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 404, { error: "not found" }));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerGetTimeEntry], deps);
  try {
    const result = await client.callTool({ name: "get_time_entry", arguments: { id: 999 } });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string; notFound: boolean; status: number } };
    assert.equal(parsed.error.type, "toggl_api");
    assert.equal(parsed.error.notFound, true);
    assert.equal(parsed.error.status, 404);
  } finally {
    await close();
    await fake.close();
  }
});

test("invalid (non-positive) id is rejected before any Toggl request", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 200, {}));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerGetTimeEntry], deps);
  try {
    const result = await client.callTool({ name: "get_time_entry", arguments: { id: -1 } });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("a stale-cache warning is passed through on the curated result", async () => {
  const dir = await fs.mkdtemp(path.join((await import("node:os")).tmpdir(), "toggl-mcp-warn-"));
  const cachePath = path.join(dir, "projects.json");
  const eightDaysAgo = new Date(Date.now() - 8 * 24 * 60 * 60 * 1000).toISOString();
  await fs.writeFile(
    cachePath,
    JSON.stringify({ fetchedAt: eightDaysAgo, projects: [{ id: 3, name: "[TOGGL-3] Gamma", active: true, workspaceId: 99 }] }),
  );

  let entryRequests = 0;
  const fake = await startFakeToggl((req, res) => {
    if (req.url?.startsWith("/me/projects")) {
      sendJson(res, 500, { error: "boom" });
      return;
    }
    entryRequests += 1;
    sendJson(res, 200, { id: 7, description: "[TOGGL-3] work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T02:00:00Z", project_id: 3 });
  });
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerGetTimeEntry], deps);
  try {
    const result = await client.callTool({ name: "get_time_entry", arguments: { id: 7 } });
    assert.equal(result.isError, undefined);
    assert.equal(entryRequests, 1);
    const parsed = parseResult(result) as { project: string | null; warnings: { type: string }[] };
    assert.equal(parsed.project, "[TOGGL-3] Gamma");
    assert.equal(parsed.warnings.length, 1);
    assert.equal(parsed.warnings[0].type, "stale_cache");
  } finally {
    await close();
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});
