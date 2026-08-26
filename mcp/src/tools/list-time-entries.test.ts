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
import type { ToolDeps } from "./deps.js";

function makeDeps(fake: FakeTogglServer, cachePath: string): ToolDeps {
  const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
  const config = loadConfig({ TOGGL_API_TOKEN: "tok", TOGGL_WORKSPACE_ID: "99" });
  return { client, cachePath, config };
}

test("list_time_entries issues exactly one Toggl request per call, regardless of range width", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => {
      sendJson(res, 200, [
        { id: 1, description: "no project", start: "2020-01-01T00:00:00Z", stop: "2020-01-01T01:00:00Z", workspace_id: 99 },
      ]);
    });
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2020-01-01", end_date: "2020-12-31" },
    });
    assert.equal(result.isError, undefined);
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].url, "/me/time_entries?start_date=2020-01-01&end_date=2020-12-31");
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("invalid start_date format is rejected before any Toggl request", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => sendJson(res, 200, []));
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "not-a-date", end_date: "2026-01-02" },
    });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("end_date before start_date is rejected before any Toggl request", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => sendJson(res, 200, []));
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-10", end_date: "2026-01-01" },
    });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("result entries carry exactly the five curated fields; foreign-workspace entries are omitted", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => {
      sendJson(res, 200, [
        { id: 1, description: "mine", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", workspace_id: 99, project_id: null },
        { id: 2, description: "not mine", start: "2026-01-02T00:00:00Z", stop: "2026-01-02T01:00:00Z", workspace_id: 7 },
      ]);
    });
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31" },
    });
    const parsed = parseResult(result) as { entries: unknown[] };
    assert.equal(parsed.entries.length, 1);
    assert.deepEqual(parsed.entries[0], {
      id: 1,
      description: "mine",
      start: "2026-01-01T00:00:00Z",
      stop: "2026-01-01T01:00:00Z",
      project: null,
    });
    assert.deepEqual(Object.keys(parsed.entries[0] as object).sort(), ["description", "id", "project", "start", "stop"]);
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("an empty range returns a successful empty array, not an error", async () => {
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => sendJson(res, 200, []));
    const cachePath = makeTmpCachePath();
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-01" },
    });
    assert.equal(result.isError, undefined);
    assert.deepEqual(parseResult(result), { entries: [] });
  } finally {
    if (close) await close();
    if (fake) await fake.close();
  }
});

test("workspace_id param overrides TOGGL_WORKSPACE_ID for filtering", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => {
      sendJson(res, 200, [
        { id: 1, description: "other ws", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", workspace_id: 55 },
      ]);
    });
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31", workspace_id: 55 },
    });
    const parsed = parseResult(result) as { entries: unknown[] };
    assert.equal(parsed.entries.length, 1);
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("entries with a project_id resolve project names via a pre-warmed cache, with exactly one Toggl request", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => {
      sendJson(res, 200, [
        { id: 1, description: "[TOGGL-1] work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", workspace_id: 99, project_id: 5 },
      ]);
    });
    writeProjectCache(cachePath, [{ id: 5, name: "[TOGGL-1] Alpha", active: true, workspaceId: 99 }]);
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31" },
    });
    const parsed = parseResult(result) as { entries: { project: string | null }[] };
    assert.equal(parsed.entries[0].project, "[TOGGL-1] Alpha");
    assert.equal(fake.requests.length, 1);
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("Toggl 500 error is surfaced as a structured error, not a crash", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => sendJson(res, 500, { error: "boom" }));
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31" },
    });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string; status: number } };
    assert.equal(parsed.error.type, "toggl_api");
    assert.equal(parsed.error.status, 500);
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("date range exceeding 366 days is rejected before any Toggl request", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => sendJson(res, 200, []));
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2027-12-31" },
    });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("invalid workspace_id is rejected before any Toggl request", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => sendJson(res, 200, []));
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31", workspace_id: 0 },
    });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("time entries fetched with no pre-written cache, projects resolved from fresh fetch, exactly two requests", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    let requestCount = 0;
    fake = await startFakeToggl((_req, res) => {
      requestCount += 1;
      if (requestCount === 1) {
        sendJson(res, 200, [
          { id: 1, description: "task", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", workspace_id: 99, project_id: 7 },
        ]);
      } else {
        sendJson(res, 200, [{ id: 7, name: "Project Seven", active: true, workspace_id: 99 }]);
      }
    });
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31" },
    });
    assert.equal(result.isError, undefined);
    const parsed = parseResult(result) as { entries: { project: string | null }[] };
    assert.equal(parsed.entries[0].project, "Project Seven");
    assert.equal(fake.requests.length, 2);
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("when listTimeEntries succeeds but projects fetch fails, error is returned", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    let requestCount = 0;
    fake = await startFakeToggl((_req, res) => {
      requestCount += 1;
      if (requestCount === 1) {
        sendJson(res, 200, [
          { id: 1, description: "task", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", workspace_id: 99, project_id: 7 },
        ]);
      } else {
        sendJson(res, 500, { error: "projects unavailable" });
      }
    });
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31" },
    });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string } };
    assert.equal(parsed.error.type, "toggl_api");
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("stale cache with failed refresh returns entries with stale_cache warning", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((_req, res) => {
      if (new URL((_req.url ?? ""), "http://localhost").pathname === "/me/time_entries") {
        sendJson(res, 200, [
          { id: 1, description: "task", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", workspace_id: 99, project_id: 7 },
        ]);
      } else {
        sendJson(res, 500, { error: "projects unavailable" });
      }
    });
    const eightDaysAgo = new Date(Date.now() - 8 * 24 * 60 * 60 * 1000).toISOString();
    writeProjectCache(cachePath, [{ id: 7, name: "Project Seven", active: true, workspaceId: 99 }], eightDaysAgo);
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31" },
    });
    assert.equal(result.isError, undefined);
    const parsed = parseResult(result) as { entries: { project: string | null }[]; warnings?: unknown[] };
    assert.equal(parsed.entries[0].project, "Project Seven");
    assert.ok(parsed.warnings);
    assert.equal(parsed.warnings.length, 1);
    const warning = parsed.warnings[0] as { type: string };
    assert.equal(warning.type, "stale_cache");
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("network error during time entries fetch is surfaced as network error type", async () => {
  const cachePath = makeTmpCachePath();
  let fake: FakeTogglServer | undefined;
  let close: (() => Promise<void>) | undefined;
  try {
    fake = await startFakeToggl((req, res) => {
      req.socket.destroy();
    });
    const deps = makeDeps(fake, cachePath);
    const connected = await connectToolClient([registerListTimeEntries], deps);
    close = connected.close;
    const result = await connected.client.callTool({
      name: "list_time_entries",
      arguments: { start_date: "2026-01-01", end_date: "2026-01-31" },
    });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string } };
    assert.equal(parsed.error.type, "network");
  } finally {
    if (close) await close();
    if (fake) await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});
