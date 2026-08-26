import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { registerListTimeEntries } from "./list-time-entries.js";
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

test("list_time_entries issues exactly one Toggl request per call, regardless of range width", async () => {
  const fake = await startFakeToggl((_req, res) => {
    sendJson(res, 200, [
      { id: 1, description: "no project", start: "2020-01-01T00:00:00Z", stop: "2020-01-01T01:00:00Z", workspace_id: 99 },
    ]);
  });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerListTimeEntries], deps);
  try {
    const result = await client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2000-01-01", end_date: "2026-12-31" },
    });
    assert.equal(result.isError, undefined);
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].url, "/me/time_entries?start_date=2000-01-01&end_date=2026-12-31");
  } finally {
    await close();
    await fake.close();
  }
});

test("invalid start_date format is rejected before any Toggl request", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 200, []));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerListTimeEntries], deps);
  try {
    const result = await client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "not-a-date", end_date: "2026-01-02" },
    });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("end_date before start_date is rejected before any Toggl request", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 200, []));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerListTimeEntries], deps);
  try {
    const result = await client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-10", end_date: "2026-01-01" },
    });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("result entries carry exactly the five curated fields; foreign-workspace entries are omitted", async () => {
  const fake = await startFakeToggl((_req, res) => {
    sendJson(res, 200, [
      { id: 1, description: "mine", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", workspace_id: 99, project_id: null },
      { id: 2, description: "not mine", start: "2026-01-02T00:00:00Z", stop: "2026-01-02T01:00:00Z", workspace_id: 7 },
    ]);
  });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerListTimeEntries], deps);
  try {
    const result = await client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31" },
    });
    const parsed = parseResult(result) as unknown[];
    assert.equal(parsed.length, 1);
    assert.deepEqual(parsed[0], {
      id: 1,
      description: "mine",
      start: "2026-01-01T00:00:00Z",
      stop: "2026-01-01T01:00:00Z",
      project: null,
    });
    assert.deepEqual(Object.keys(parsed[0] as object).sort(), ["description", "id", "project", "start", "stop"]);
  } finally {
    await close();
    await fake.close();
  }
});

test("an empty range returns a successful empty array, not an error", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 200, []));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerListTimeEntries], deps);
  try {
    const result = await client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-01" },
    });
    assert.equal(result.isError, undefined);
    assert.deepEqual(parseResult(result), []);
  } finally {
    await close();
    await fake.close();
  }
});

test("workspace_id param overrides TOGGL_WORKSPACE_ID for filtering", async () => {
  const fake = await startFakeToggl((_req, res) => {
    sendJson(res, 200, [
      { id: 1, description: "other ws", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", workspace_id: 55 },
    ]);
  });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerListTimeEntries], deps);
  try {
    const result = await client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31", workspace_id: 55 },
    });
    const parsed = parseResult(result) as unknown[];
    assert.equal(parsed.length, 1);
  } finally {
    await close();
    await fake.close();
  }
});

test("entries with a project_id resolve project names via a pre-warmed cache, with exactly one Toggl request", async () => {
  const fake = await startFakeToggl((_req, res) => {
    sendJson(res, 200, [
      { id: 1, description: "[TOGGL-1] work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", workspace_id: 99, project_id: 5 },
    ]);
  });
  const cachePath = makeTmpCachePath();
  writeProjectCache(cachePath, [{ id: 5, name: "[TOGGL-1] Alpha", active: true, workspaceId: 99 }]);
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerListTimeEntries], deps);
  try {
    const result = await client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31" },
    });
    const parsed = parseResult(result) as { project: string | null }[];
    assert.equal(parsed[0].project, "[TOGGL-1] Alpha");
    assert.equal(fake.requests.length, 1);
  } finally {
    await close();
    await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("Toggl 500 error is surfaced as a structured error, not a crash", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 500, { error: "boom" }));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerListTimeEntries], deps);
  try {
    const result = await client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31" },
    });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string; status: number } };
    assert.equal(parsed.error.type, "toggl_api");
    assert.equal(parsed.error.status, 500);
  } finally {
    await close();
    await fake.close();
  }
});
