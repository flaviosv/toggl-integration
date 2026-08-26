import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { registerRefreshProjects } from "./refresh-projects.js";
import { TogglClient } from "../toggl/client.js";
import { loadConfig } from "../config.js";
import { connectToolClient, makeTmpCachePath, parseResult, sendJson, startFakeToggl, writeProjectCache, type FakeTogglServer } from "./test-harness.js";
import type { ToolDeps } from "./schemas.js";

function makeDeps(fake: FakeTogglServer, cachePath: string): ToolDeps {
  const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
  const config = loadConfig({ TOGGL_API_TOKEN: "tok", TOGGL_WORKSPACE_ID: "99" });
  return { client, cachePath, config };
}

test("a fresh cache is still forcibly refetched and the cache file is overwritten", async () => {
  const fake = await startFakeToggl((_req, res) => {
    sendJson(res, 200, [
      { id: 1, name: "[teachmeto.ai] JSP", active: true, workspace_id: 99 },
      { id: 2, name: "[teachmeto.ai] OIQ", active: true, workspace_id: 99 },
    ]);
  });
  const cachePath = makeTmpCachePath();
  writeProjectCache(cachePath, [{ id: 1, name: "[teachmeto.ai] JSP", active: true, workspaceId: 99 }], new Date().toISOString());
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerRefreshProjects], deps);
  try {
    const result = await client.callTool({ name: "refresh_projects", arguments: {} });
    assert.equal(result.isError, undefined);
    assert.equal(fake.requests.length, 1);
    const written = JSON.parse(await fs.readFile(cachePath, "utf8"));
    assert.equal(written.projects.length, 2);
  } finally {
    await close();
    await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("success result reports the refetched project count and the new fetchedAt", async () => {
  const fake = await startFakeToggl((_req, res) => {
    sendJson(res, 200, [{ id: 1, name: "[teachmeto.ai] JSP", active: true, workspace_id: 99 }]);
  });
  const cachePath = makeTmpCachePath();
  const staleFetchedAt = new Date(Date.now() - 8 * 24 * 60 * 60 * 1000).toISOString();
  writeProjectCache(cachePath, [], staleFetchedAt);
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerRefreshProjects], deps);
  try {
    const result = await client.callTool({ name: "refresh_projects", arguments: {} });
    const parsed = parseResult(result) as { count: number; fetchedAt: string };
    assert.equal(parsed.count, 1);
    assert.notEqual(parsed.fetchedAt, staleFetchedAt);
    const written = JSON.parse(await fs.readFile(cachePath, "utf8"));
    assert.equal(parsed.fetchedAt, written.fetchedAt);
  } finally {
    await close();
    await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});

test("a refetch failure with no prior cache surfaces the underlying Toggl error", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 500, { error: "boom" }));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerRefreshProjects], deps);
  try {
    const result = await client.callTool({ name: "refresh_projects", arguments: {} });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string; status: number } };
    assert.equal(parsed.error.type, "toggl_api");
    assert.equal(parsed.error.status, 500);
  } finally {
    await close();
    await fake.close();
  }
});

test("a refetch failure with a prior cache returns the stale count with a stale_cache warning, not an error", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 500, { error: "boom" }));
  const cachePath = makeTmpCachePath();
  const staleFetchedAt = new Date(Date.now() - 8 * 24 * 60 * 60 * 1000).toISOString();
  writeProjectCache(cachePath, [{ id: 1, name: "[teachmeto.ai] JSP", active: true, workspaceId: 99 }], staleFetchedAt);
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerRefreshProjects], deps);
  try {
    const result = await client.callTool({ name: "refresh_projects", arguments: {} });
    assert.equal(result.isError, undefined);
    const parsed = parseResult(result) as { count: number; warnings: { type: string }[] };
    assert.equal(parsed.count, 1);
    assert.equal(parsed.warnings.length, 1);
    assert.equal(parsed.warnings[0].type, "stale_cache");
  } finally {
    await close();
    await fake.close();
    await fs.rm(path.dirname(cachePath), { recursive: true, force: true });
  }
});
