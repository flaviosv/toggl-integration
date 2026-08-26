import { test } from "node:test";
import assert from "node:assert/strict";
import { registerCreateTimeEntry } from "./create-time-entry.js";
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

function projectsRoutedFake(createResponse: unknown): Promise<FakeTogglServer> {
  return startFakeToggl((req, res) => {
    if (req.url?.startsWith("/me/projects")) {
      sendJson(res, 500, { error: "projects endpoint should not be called" });
      return;
    }
    sendJson(res, 200, createResponse);
  });
}

test("invalid start (bad RFC3339) is rejected before any Toggl request", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 200, {}));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerCreateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "create_time_entry",
      arguments: { description: "work", start: "not-a-timestamp", stop: "2026-01-01T02:00:00Z" },
    });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("stop not strictly after start is rejected before any Toggl request", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 200, {}));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerCreateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "create_time_entry",
      arguments: { description: "work", start: "2026-01-01T02:00:00Z", stop: "2026-01-01T02:00:00Z" },
    });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("[JSP-10] description over a 2-hour window: POST carries the matched project id and duration 7200", async () => {
  const fake = await startFakeToggl((req, res) => {
    if (req.url?.startsWith("/me/projects")) {
      sendJson(res, 200, []);
      return;
    }
    sendJson(res, 200, { id: 1, description: "[JSP-10] Creating form", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T02:00:00Z", project_id: 10 });
  });
  const cachePath = makeTmpCachePath();
  writeProjectCache(cachePath, [{ id: 10, name: "[teachmeto.ai] JSP", active: true, workspaceId: 99 }]);
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerCreateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "create_time_entry",
      arguments: { description: "[JSP-10] Creating form", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T02:00:00Z" },
    });
    assert.equal(result.isError, undefined);
    assert.equal(fake.requests.length, 1);
    const body = JSON.parse(fake.requests[0].body);
    assert.equal(body.project_id, 10);
    assert.equal(body.duration, 7200);
    assert.equal(body.description, "[JSP-10] Creating form");
    assert.equal(body.workspace_id, 99);
    assert.equal(body.created_with, "toggl-mcp");
  } finally {
    await close();
    await fake.close();
  }
});

test("ambiguous ticket code returns the matching error and issues zero POST requests", async () => {
  const fake = await startFakeToggl((req, res) => {
    if (req.url?.startsWith("/me/projects")) {
      sendJson(res, 200, []);
      return;
    }
    sendJson(res, 200, { id: 1 });
  });
  const cachePath = makeTmpCachePath();
  writeProjectCache(cachePath, [
    { id: 10, name: "[teachmeto.ai] JSP", active: true, workspaceId: 99 },
    { id: 11, name: "[other-client] JSP", active: true, workspaceId: 99 },
  ]);
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerCreateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "create_time_entry",
      arguments: { description: "[JSP-10] Creating form", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T02:00:00Z" },
    });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string } };
    assert.equal(parsed.error.type, "matching");
    assert.equal(fake.requests.filter((r) => r.method === "POST").length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("no-match ticket code returns the matching error and issues zero POST requests", async () => {
  const fake = await startFakeToggl((req, res) => {
    if (req.url?.startsWith("/me/projects")) {
      sendJson(res, 200, []);
      return;
    }
    sendJson(res, 200, { id: 1 });
  });
  const cachePath = makeTmpCachePath();
  writeProjectCache(cachePath, [{ id: 10, name: "[teachmeto.ai] OIQ", active: true, workspaceId: 99 }]);
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerCreateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "create_time_entry",
      arguments: { description: "[JSP-10] Creating form", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T02:00:00Z" },
    });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string } };
    assert.equal(parsed.error.type, "matching");
    assert.equal(fake.requests.filter((r) => r.method === "POST").length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("explicit project_id bypasses matching entirely: zero requests to the projects endpoint", async () => {
  const fake = await projectsRoutedFake({ id: 1, description: "work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", project_id: 42 });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerCreateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "create_time_entry",
      arguments: { description: "[JSP-10] work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", project_id: 42 },
    });
    assert.equal(result.isError, undefined);
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].method, "POST");
    const body = JSON.parse(fake.requests[0].body);
    assert.equal(body.project_id, 42);
  } finally {
    await close();
    await fake.close();
  }
});

test("untagged description with no project_id omits project_id from the POST body and reports project: null", async () => {
  const fake = await projectsRoutedFake({ id: 1, description: "untagged work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", project_id: null });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerCreateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "create_time_entry",
      arguments: { description: "untagged work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z" },
    });
    assert.equal(result.isError, undefined);
    const body = JSON.parse(fake.requests[0].body);
    assert.equal("project_id" in body, false);
    const parsed = parseResult(result) as { project: string | null };
    assert.equal(parsed.project, null);
  } finally {
    await close();
    await fake.close();
  }
});

test("omitted workspace_id uses TOGGL_WORKSPACE_ID", async () => {
  const fake = await projectsRoutedFake({ id: 1, description: "work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z" });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerCreateTimeEntry], deps);
  try {
    await client.callTool({
      name: "create_time_entry",
      arguments: { description: "work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z" },
    });
    assert.equal(fake.requests[0].url, "/workspaces/99/time_entries");
    const body = JSON.parse(fake.requests[0].body);
    assert.equal(body.workspace_id, 99);
  } finally {
    await close();
    await fake.close();
  }
});

test("a supplied workspace_id overrides TOGGL_WORKSPACE_ID for that call only", async () => {
  const fake = await projectsRoutedFake({ id: 1, description: "work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z" });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerCreateTimeEntry], deps);
  try {
    await client.callTool({
      name: "create_time_entry",
      arguments: { description: "work", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z", workspace_id: 777 },
    });
    assert.equal(fake.requests[0].url, "/workspaces/777/time_entries");
    const body = JSON.parse(fake.requests[0].body);
    assert.equal(body.workspace_id, 777);
    assert.equal(deps.config.togglWorkspaceId, 99);
  } finally {
    await close();
    await fake.close();
  }
});

test("a Toggl 500 on create is surfaced as a structured error", async () => {
  const fake = await startFakeToggl((req, res) => {
    if (req.url?.startsWith("/me/projects")) {
      sendJson(res, 200, []);
      return;
    }
    sendJson(res, 500, { error: "boom" });
  });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerCreateTimeEntry], deps);
  try {
    const result = await client.callTool({
      name: "create_time_entry",
      arguments: { description: "untagged", start: "2026-01-01T00:00:00Z", stop: "2026-01-01T01:00:00Z" },
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
