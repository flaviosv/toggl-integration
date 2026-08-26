import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import fsSync from "node:fs";
import os from "node:os";
import path from "node:path";
import http from "node:http";
import type { AddressInfo } from "node:net";
import { getProjects } from "./project-cache.js";
import { TogglClient } from "../toggl/client.js";

interface FakeServer {
  baseUrl: string;
  requestCount: number;
  close: () => Promise<void>;
}

function startProjectsServer(
  responder: (res: http.ServerResponse) => void,
): Promise<FakeServer> {
  const state = { requestCount: 0 };
  const server = http.createServer((_req, res) => {
    state.requestCount += 1;
    responder(res);
  });
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address() as AddressInfo;
      resolve({
        baseUrl: `http://127.0.0.1:${port}`,
        get requestCount() {
          return state.requestCount;
        },
        close: () => new Promise((r) => server.close(() => r())),
      });
    });
  });
}

function sendJson(res: http.ServerResponse, status: number, payload: unknown): void {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(JSON.stringify(payload));
}

function makeTmpDir(): string {
  return fsSync.mkdtempSync(path.join(os.tmpdir(), "toggl-mcp-cache-test-"));
}

const RAW_PROJECTS_A = [{ id: 1, name: "[TOGGL-1] Alpha", active: true, workspace_id: 99 }];
const RAW_PROJECTS_B = [{ id: 2, name: "[TOGGL-2] Beta", active: true, workspace_id: 99 }];
const CACHED_A = [{ id: 1, name: "[TOGGL-1] Alpha", active: true, workspaceId: 99 }];
const CACHED_B = [{ id: 2, name: "[TOGGL-2] Beta", active: true, workspaceId: 99 }];

test("no cache file: one listProjects call, cache written with fetchedAt and projects", async () => {
  const fake = await startProjectsServer((res) => sendJson(res, 200, RAW_PROJECTS_A));
  const dir = makeTmpDir();
  try {
    const cachePath = path.join(dir, "projects.json");
    const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
    const result = await getProjects(client, cachePath);
    assert.equal(fake.requestCount, 1);
    assert.deepEqual(result.projects, CACHED_A);
    assert.equal(result.warning, undefined);
    const written = JSON.parse(await fs.readFile(cachePath, "utf8"));
    assert.deepEqual(written.projects, CACHED_A);
    assert.equal(typeof written.fetchedAt, "string");
    assert.ok(!Number.isNaN(Date.parse(written.fetchedAt)));
  } finally {
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test("fresh cache (<7 days): zero listProjects calls, returns cached projects verbatim", async () => {
  const fake = await startProjectsServer((res) => sendJson(res, 200, RAW_PROJECTS_B));
  const dir = makeTmpDir();
  try {
    const cachePath = path.join(dir, "projects.json");
    await fs.writeFile(
      cachePath,
      JSON.stringify({ fetchedAt: new Date().toISOString(), projects: CACHED_A }),
    );
    const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
    const result = await getProjects(client, cachePath);
    assert.equal(fake.requestCount, 0);
    assert.deepEqual(result.projects, CACHED_A);
    assert.equal(result.warning, undefined);
  } finally {
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test("stale cache (>=7 days): one refetch, cache overwritten on disk", async () => {
  const fake = await startProjectsServer((res) => sendJson(res, 200, RAW_PROJECTS_B));
  const dir = makeTmpDir();
  try {
    const cachePath = path.join(dir, "projects.json");
    const eightDaysAgo = new Date(Date.now() - 8 * 24 * 60 * 60 * 1000).toISOString();
    await fs.writeFile(cachePath, JSON.stringify({ fetchedAt: eightDaysAgo, projects: CACHED_A }));
    const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
    const result = await getProjects(client, cachePath);
    assert.equal(fake.requestCount, 1);
    assert.deepEqual(result.projects, CACHED_B);
    const written = JSON.parse(await fs.readFile(cachePath, "utf8"));
    assert.deepEqual(written.projects, CACHED_B);
    assert.notEqual(written.fetchedAt, eightDaysAgo);
  } finally {
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test("forceRefresh true always refetches, even on a fresh cache", async () => {
  const fake = await startProjectsServer((res) => sendJson(res, 200, RAW_PROJECTS_B));
  const dir = makeTmpDir();
  try {
    const cachePath = path.join(dir, "projects.json");
    await fs.writeFile(
      cachePath,
      JSON.stringify({ fetchedAt: new Date().toISOString(), projects: CACHED_A }),
    );
    const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
    const result = await getProjects(client, cachePath, { forceRefresh: true });
    assert.equal(fake.requestCount, 1);
    assert.deepEqual(result.projects, CACHED_B);
  } finally {
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test("malformed JSON cache file: treated as miss, refetches, never throws the parse error", async () => {
  const fake = await startProjectsServer((res) => sendJson(res, 200, RAW_PROJECTS_A));
  const dir = makeTmpDir();
  try {
    const cachePath = path.join(dir, "projects.json");
    await fs.writeFile(cachePath, "{ not valid json ");
    const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
    const result = await getProjects(client, cachePath);
    assert.equal(fake.requestCount, 1);
    assert.deepEqual(result.projects, CACHED_A);
  } finally {
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test("cache file missing required fetchedAt/projects fields: treated as miss, refetches", async () => {
  const fake = await startProjectsServer((res) => sendJson(res, 200, RAW_PROJECTS_A));
  const dir = makeTmpDir();
  try {
    const cachePath = path.join(dir, "projects.json");
    await fs.writeFile(cachePath, JSON.stringify({ someOtherField: true }));
    const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
    const result = await getProjects(client, cachePath);
    assert.equal(fake.requestCount, 1);
    assert.deepEqual(result.projects, CACHED_A);
  } finally {
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test("unwritable cache directory: write failure logged via logger, freshly-fetched list still returned, call succeeds", async (t) => {
  const errorMock = t.mock.method(console, "error", () => {});
  const fake = await startProjectsServer((res) => sendJson(res, 200, RAW_PROJECTS_A));
  const dir = makeTmpDir();
  try {
    const cachePath = path.join(dir, "projects.json");
    fsSync.chmodSync(dir, 0o555);
    const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
    const result = await getProjects(client, cachePath);
    assert.equal(fake.requestCount, 1);
    assert.deepEqual(result.projects, CACHED_A);
    assert.equal(result.warning, undefined);
    assert.ok(errorMock.mock.callCount() >= 1);
    const line = JSON.parse(errorMock.mock.calls[0].arguments[0] as string);
    assert.equal(line.level, "error");
    assert.equal(typeof line.message, "string");
  } finally {
    fsSync.chmodSync(dir, 0o755);
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test("stale cache + refetch fails (500) + stale cache exists: returns stale projects plus stale_cache warning", async () => {
  const fake = await startProjectsServer((res) => sendJson(res, 500, { error: "boom" }));
  const dir = makeTmpDir();
  try {
    const cachePath = path.join(dir, "projects.json");
    const eightDaysAgo = new Date(Date.now() - 8 * 24 * 60 * 60 * 1000).toISOString();
    await fs.writeFile(cachePath, JSON.stringify({ fetchedAt: eightDaysAgo, projects: CACHED_A }));
    const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
    const result = await getProjects(client, cachePath);
    assert.equal(fake.requestCount, 1);
    assert.deepEqual(result.projects, CACHED_A);
    assert.ok(result.warning);
    assert.equal(result.warning?.type, "stale_cache");
    assert.ok(result.warning && result.warning.cacheAgeSeconds >= 8 * 24 * 60 * 60 - 5);
    assert.equal(typeof result.warning?.underlyingError, "string");
  } finally {
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test("no cache at all + refetch fails: rethrows the underlying TogglApiError unchanged", async () => {
  const fake = await startProjectsServer((res) => sendJson(res, 500, { error: "boom" }));
  const dir = makeTmpDir();
  try {
    const cachePath = path.join(dir, "projects.json");
    const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
    await assert.rejects(
      () => getProjects(client, cachePath),
      (err: unknown) => {
        assert.equal((err as { name: string }).name, "TogglApiError");
        assert.equal((err as { status: number }).status, 500);
        return true;
      },
    );
  } finally {
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});

test("after a successful write, no leftover .tmp-* file remains in the cache directory", async () => {
  const fake = await startProjectsServer((res) => sendJson(res, 200, RAW_PROJECTS_A));
  const dir = makeTmpDir();
  try {
    const cachePath = path.join(dir, "projects.json");
    const client = new TogglClient({ apiToken: "tok", baseUrl: fake.baseUrl });
    await getProjects(client, cachePath);
    const entries = await fs.readdir(dir);
    const tmpFiles = entries.filter((f) => f.includes(".tmp-"));
    assert.deepEqual(tmpFiles, []);
    assert.deepEqual(entries, ["projects.json"]);
  } finally {
    await fake.close();
    await fs.rm(dir, { recursive: true, force: true });
  }
});
