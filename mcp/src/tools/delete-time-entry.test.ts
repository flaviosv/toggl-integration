import { test } from "node:test";
import assert from "node:assert/strict";
import { registerDeleteTimeEntry } from "./delete-time-entry.js";
import { TogglClient } from "../toggl/client.js";
import { loadConfig } from "../config.js";
import { connectToolClient, makeTmpCachePath, parseResult, sendJson, startFakeToggl, type FakeTogglServer } from "./test-harness.js";
import type { ToolDeps } from "./schemas.js";

function makeDeps(fake: FakeTogglServer, cachePath: string): ToolDeps {
  const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
  const config = loadConfig({ TOGGL_API_TOKEN: "tok", TOGGL_WORKSPACE_ID: "99" });
  return { client, cachePath, config };
}

test("deleting an existing id issues exactly one DELETE and returns a success result naming that id", async () => {
  const fake = await startFakeToggl((_req, res) => {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end();
  });
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerDeleteTimeEntry], deps);
  try {
    const result = await client.callTool({ name: "delete_time_entry", arguments: { id: 7 } });
    assert.equal(result.isError, undefined);
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].method, "DELETE");
    assert.equal(fake.requests[0].url, "/workspaces/99/time_entries/7");
    assert.deepEqual(parseResult(result), { deleted: true, id: 7 });
  } finally {
    await close();
    await fake.close();
  }
});

test("deleting an id the fake server 404s returns a structured not-found error, not a success", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 404, { error: "not found" }));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerDeleteTimeEntry], deps);
  try {
    const result = await client.callTool({ name: "delete_time_entry", arguments: { id: 999 } });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string; notFound: boolean } };
    assert.equal(parsed.error.type, "toggl_api");
    assert.equal(parsed.error.notFound, true);
  } finally {
    await close();
    await fake.close();
  }
});

test("invalid (non-positive) id is rejected before any Toggl request", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 200, {}));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerDeleteTimeEntry], deps);
  try {
    const result = await client.callTool({ name: "delete_time_entry", arguments: { id: 0 } });
    assert.equal(result.isError, true);
    assert.equal(fake.requests.length, 0);
  } finally {
    await close();
    await fake.close();
  }
});

test("a Toggl 500 on delete is surfaced as a structured error", async () => {
  const fake = await startFakeToggl((_req, res) => sendJson(res, 500, { error: "boom" }));
  const cachePath = makeTmpCachePath();
  const deps = makeDeps(fake, cachePath);
  const { client, close } = await connectToolClient([registerDeleteTimeEntry], deps);
  try {
    const result = await client.callTool({ name: "delete_time_entry", arguments: { id: 7 } });
    assert.equal(result.isError, true);
    const parsed = parseResult(result) as { error: { type: string; status: number } };
    assert.equal(parsed.error.type, "toggl_api");
    assert.equal(parsed.error.status, 500);
  } finally {
    await close();
    await fake.close();
  }
});
