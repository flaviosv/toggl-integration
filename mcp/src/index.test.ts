import { test } from "node:test";
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import fsSync from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ENTRY_POINT = path.join(__dirname, "index.js");
const MCP_ROOT = path.join(__dirname, "..");
const ENV_FILE_PATH = path.join(MCP_ROOT, ".env");

function tmpCachePath(): string {
  const dir = fsSync.mkdtempSync(path.join(os.tmpdir(), "toggl-mcp-bootstrap-"));
  return path.join(dir, "projects.json");
}

test("missing TOGGL_API_TOKEN: non-zero exit, stderr names the variable, zero bytes on stdout", async () => {
  const child = spawn(process.execPath, [ENTRY_POINT], {
    env: { PATH: process.env.PATH ?? "", TOGGL_WORKSPACE_ID: "99", TOGGL_CACHE_PATH: tmpCachePath() },
    stdio: ["ignore", "pipe", "pipe"],
  });

  const stdoutChunks: Buffer[] = [];
  const stderrChunks: Buffer[] = [];
  child.stdout.on("data", (c: Buffer) => stdoutChunks.push(c));
  child.stderr.on("data", (c: Buffer) => stderrChunks.push(c));

  const exitCode: number | null = await new Promise((resolve) => {
    child.on("exit", (code) => resolve(code));
  });

  assert.notEqual(exitCode, 0);
  const stdout = Buffer.concat(stdoutChunks);
  const stderr = Buffer.concat(stderrChunks).toString("utf8");
  assert.equal(stdout.length, 0);
  assert.match(stderr, /TOGGL_API_TOKEN/);
});

test("invalid TOGGL_WORKSPACE_ID: non-zero exit, stderr names the variable, zero bytes on stdout", async () => {
  const child = spawn(process.execPath, [ENTRY_POINT], {
    env: {
      PATH: process.env.PATH ?? "",
      TOGGL_API_TOKEN: "dummy-token",
      TOGGL_WORKSPACE_ID: "not-a-number",
      TOGGL_CACHE_PATH: tmpCachePath(),
    },
    stdio: ["ignore", "pipe", "pipe"],
  });

  const stdoutChunks: Buffer[] = [];
  const stderrChunks: Buffer[] = [];
  child.stdout.on("data", (c: Buffer) => stdoutChunks.push(c));
  child.stderr.on("data", (c: Buffer) => stderrChunks.push(c));

  const exitCode: number | null = await new Promise((resolve) => {
    child.on("exit", (code) => resolve(code));
  });

  assert.notEqual(exitCode, 0);
  assert.equal(Buffer.concat(stdoutChunks).length, 0);
  assert.match(Buffer.concat(stderrChunks).toString("utf8"), /TOGGL_WORKSPACE_ID/);
});

test("valid env: successful MCP handshake and a tool list of exactly the 1 registered tool", async () => {
  const transport = new StdioClientTransport({
    command: process.execPath,
    args: [ENTRY_POINT],
    env: {
      TOGGL_API_TOKEN: "dummy-token",
      TOGGL_WORKSPACE_ID: "99",
      TOGGL_CACHE_PATH: tmpCachePath(),
    },
  });
  const client = new Client({ name: "bootstrap-test-client", version: "0.0.0" });
  try {
    await client.connect(transport);
    const { tools } = await client.listTools();
    const names = tools.map((t) => t.name).sort();
    assert.deepEqual(names, ["list_time_entries"]);
  } finally {
    await client.close();
  }
});

test(".env file loading: credentials from .env file (not passed via env) enable successful handshake", async () => {
  const envContent = `TOGGL_API_TOKEN=env-file-token\nTOGGL_WORKSPACE_ID=88\n`;
  const cachePath = tmpCachePath();
  fsSync.writeFileSync(ENV_FILE_PATH, envContent, "utf8");

  try {
    const transport = new StdioClientTransport({
      command: process.execPath,
      args: [ENTRY_POINT],
      env: {
        PATH: process.env.PATH ?? "",
        TOGGL_CACHE_PATH: cachePath,
      },
    });
    const client = new Client({ name: "env-file-test-client", version: "0.0.0" });
    try {
      await client.connect(transport);
      const { tools } = await client.listTools();
      const names = tools.map((t) => t.name).sort();
      assert.deepEqual(names, ["list_time_entries"]);
    } finally {
      await client.close();
    }
  } finally {
    fsSync.rmSync(ENV_FILE_PATH, { force: true });
  }
});
