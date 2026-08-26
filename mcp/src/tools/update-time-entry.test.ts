import { test } from "node:test";
import assert from "node:assert/strict";
import { registerUpdateTimeEntry } from "./update-time-entry.js";
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

const EXISTING_ENTRY = {
  id: 7,
  description: "[JSP-10] a",
  start: "2026-01-01T00:00:00Z",
  stop: "2026-01-01T02:00:00Z",
  project_id: 10,
  workspace_id: 99,
};

function routedFake(putResponse: unknown): Promise<FakeTogglServer> {
  return startFakeToggl((req, res) => {
    if (req.method === "GET" && req.url?.startsWith("/me/time_entries/")) {
      sendJson(res, 200, EXISTING_ENTRY);
      return;
    }
    if (req.url?.startsWith("/me/projects")) {
      sendJson(res, 500, { error: "projects endpoint should not be called" });
      return;
    }
    if (req.method === "PUT") {
      sendJson(res, 200, putResponse);
      return;
    }
    sendJson(res, 404, { error: "unexpected" });
  });
}

test("a call supplying only id is rejected before any Toggl request", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 200, {}));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerUpdateTimeEntry], deps);
  try {
    const result = await client.callTool({ name: "update_time_entry", arguments: { id: 7 } });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("updating only description [JSP-10] a -> [OIQ-3] b carries the OIQ project id, keeps original start/stop, exactly two requests", async () => {
  const fake = await startFakeToggl((req, res) => {
    if (req.method === "GET" && req.url?.startsWith("/me/time_entries/")) {
      sendJson(res, 200, EXISTING_ENTRY);
      return;
    }
    if (req.method === "PUT") {
      sendJson(res, 200, { ...EXISTING_ENTRY, description: "[OIQ-3] b", project_id: 20 });
      return;
    }
    sendJson(res, 500, { error: "projects endpoint should not be called" });
  });
  const cachePath = makeTmpCachePath();
  writeProjectCache(cachePath, [
    { id: 10, name: "[teachmeto.ai] JSP", active: true, workspaceId: 99 },
    { id: 20, name: "[teachmeto.ai] OIQ", active: true, workspaceId: 99 },
  ]);
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerUpdateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "update_time_entry",
      arguments: { id: 7, description: "[OIQ-3] b" },
    });
    assert.equal(result.isError, undefined);
    assert.equal(fake.requests.length, 2);
    assert.equal(fake.requests[0].method, "GET");
    assert.equal(fake.requests[1].method, "PUT");
    const body = JSON.parse(fake.requests[1].body);
    assert.equal(body.project_id, 20);
    assert.equal(body.start, "2026-01-01T00:00:00Z");
    assert.equal(body.stop, "2026-01-01T02:00:00Z");
    assert.equal(body.description, "[OIQ-3] b");
  } finally {
    await close();
    await fake.close();
  }
});

test("updating start/stop without description leaves the existing project_id untouched and never reads the cache", async () => {
  const fake = await routedFake({ ...EXISTING_ENTRY, start: "2026-01-01T03:00:00Z", stop: "2026-01-01T04:00:00Z" });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerUpdateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "update_time_entry",
      arguments: { id: 7, start: "2026-01-01T03:00:00Z", stop: "2026-01-01T04:00:00Z" },
    });
    assert.equal(result.isError, undefined);
    assert.equal(fake.requests.length, 2);
    const body = JSON.parse(fake.requests[1].body);
    assert.equal(body.project_id, 10);
    assert.equal(body.duration, 3600);
  } finally {
    await close();
    await fake.close();
  }
});

test("a merged stop not strictly after the merged start is rejected and no PUT is issued", async () => {
  const fake = await routedFake(EXISTING_ENTRY);
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerUpdateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "update_time_entry",
      arguments: { id: 7, stop: "2025-01-01T00:00:00Z" },
    });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string } };
    assert.equal(parsed.error.type, "validation");
    assert.equal(fake.requests.filter((r) => r.method === "PUT").length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("a successful update returns the curated five-field shape", async () => {
  const fake = await routedFake({ ...EXISTING_ENTRY, description: "[JSP-10] updated" });
  const cachePath = makeTmpCachePath();
  writeProjectCache(cachePath, [{ id: 10, name: "[teachmeto.ai] JSP", active: true, workspaceId: 99 }]);
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerUpdateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "update_time_entry",
      arguments: { id: 7, description: "[JSP-10] updated" },
    });
    const parsed = parseResult(result) as Record<string, unknown>;
    assert.deepEqual(Object.keys(parsed).sort(), ["description", "id", "project", "start", "stop"]);
    assert.equal(parsed.project, "[teachmeto.ai] JSP");
  } finally {
    await close();
    await fake.close();
  }
});

test("explicit project_id bypasses matching even when description also changes; cache never read", async () => {
  const fake = await routedFake({ ...EXISTING_ENTRY, project_id: 42 });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerUpdateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "update_time_entry",
      arguments: { id: 7, description: "[JSP-10] anything", project_id: 42 },
    });
    assert.equal(result.isError, undefined);
    const body = JSON.parse(fake.requests[1].body);
    assert.equal(body.project_id, 42);
  } finally {
    await close();
    await fake.close();
  }
});

test("an untagged description clears the project match", async () => {
  const fake = await routedFake({ ...EXISTING_ENTRY, description: "untagged", project_id: null });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerUpdateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "update_time_entry",
      arguments: { id: 7, description: "untagged" },
    });
    assert.equal(result.isError, undefined);
    const body = JSON.parse(fake.requests[1].body);
    assert.equal(body.project_id, null);
    const parsed = parseResult(result) as { project: string | null };
    assert.equal(parsed.project, null);
  } finally {
    await close();
    await fake.close();
  }
});

test("an ambiguous re-match on description change returns the matching error and issues no PUT", async () => {
  const fake = await startFakeToggl((req, res) => {
    if (req.method === "GET" && req.url?.startsWith("/me/time_entries/")) {
      sendJson(res, 200, EXISTING_ENTRY);
      return;
    }
    sendJson(res, 200, {});
  });
  const cachePath = makeTmpCachePath();
  writeProjectCache(cachePath, [
    { id: 10, name: "[teachmeto.ai] JSP", active: true, workspaceId: 99 },
    { id: 11, name: "[other] JSP", active: true, workspaceId: 99 },
  ]);
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerUpdateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "update_time_entry",
      arguments: { id: 7, description: "[JSP-99] renamed" },
    });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string } };
    assert.equal(parsed.error.type, "matching");
    assert.equal(fake.requests.filter((r) => r.method === "PUT").length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("a 404 on the initial GET returns a structured not-found error, not an empty success", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 404, { error: "not found" }));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerUpdateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "update_time_entry",
      arguments: { id: 999, description: "x" },
    });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string; notFound: boolean } };
    assert.equal(parsed.error.type, "toggl_api");
    assert.equal(parsed.error.notFound, true);
    assert.equal(fake.requests.length, 1);
  } finally {
    await close();
    await fake.close();
  }
});
