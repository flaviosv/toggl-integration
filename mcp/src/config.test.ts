import { test } from "node:test";
import assert from "node:assert/strict";
import os from "node:os";
import path from "node:path";
import { loadConfig, ConfigError } from "./config.js";

const VALID_ENV = { TOGGL_API_TOKEN: "abc123", TOGGL_WORKSPACE_ID: "42" };

test("missing TOGGL_API_TOKEN throws ConfigError naming it", () => {
  const env = { TOGGL_WORKSPACE_ID: "42" };
  assert.throws(
    () => loadConfig(env),
    (err: unknown) => err instanceof ConfigError && /TOGGL_API_TOKEN/.test((err as Error).message),
  );
});

test("missing TOGGL_WORKSPACE_ID throws ConfigError naming it", () => {
  const env = { TOGGL_API_TOKEN: "abc123" };
  assert.throws(
    () => loadConfig(env),
    (err: unknown) => err instanceof ConfigError && /TOGGL_WORKSPACE_ID/.test((err as Error).message),
  );
});

test("non-numeric TOGGL_WORKSPACE_ID throws ConfigError naming it", () => {
  const env = { TOGGL_API_TOKEN: "abc123", TOGGL_WORKSPACE_ID: "not-a-number" };
  assert.throws(
    () => loadConfig(env),
    (err: unknown) => err instanceof ConfigError && /TOGGL_WORKSPACE_ID/.test((err as Error).message),
  );
});

test("zero TOGGL_WORKSPACE_ID throws ConfigError naming it", () => {
  const env = { TOGGL_API_TOKEN: "abc123", TOGGL_WORKSPACE_ID: "0" };
  assert.throws(
    () => loadConfig(env),
    (err: unknown) => err instanceof ConfigError && /TOGGL_WORKSPACE_ID/.test((err as Error).message),
  );
});

test("negative TOGGL_WORKSPACE_ID throws ConfigError naming it", () => {
  const env = { TOGGL_API_TOKEN: "abc123", TOGGL_WORKSPACE_ID: "-5" };
  assert.throws(
    () => loadConfig(env),
    (err: unknown) => err instanceof ConfigError && /TOGGL_WORKSPACE_ID/.test((err as Error).message),
  );
});

test("both TOGGL_API_TOKEN and TOGGL_WORKSPACE_ID missing throws one ConfigError naming both (joined, not first-error-only)", () => {
  const env = {};
  assert.throws(
    () => loadConfig(env),
    (err: unknown) =>
      err instanceof ConfigError &&
      /TOGGL_API_TOKEN/.test((err as Error).message) &&
      /TOGGL_WORKSPACE_ID/.test((err as Error).message),
  );
});

test("unset TOGGL_CACHE_PATH defaults to ~/.cache/toggl-mcp/projects.json", () => {
  const config = loadConfig(VALID_ENV);
  assert.equal(config.cachePath, path.join(os.homedir(), ".cache", "toggl-mcp", "projects.json"));
});

test("explicit TOGGL_CACHE_PATH is used verbatim", () => {
  const env = { ...VALID_ENV, TOGGL_CACHE_PATH: "/custom/path/projects.json" };
  const config = loadConfig(env);
  assert.equal(config.cachePath, "/custom/path/projects.json");
});

test("valid env produces a Config with no error thrown", () => {
  const config = loadConfig(VALID_ENV);
  assert.deepEqual(config, {
    togglApiToken: "abc123",
    togglWorkspaceId: 42,
    cachePath: path.join(os.homedir(), ".cache", "toggl-mcp", "projects.json"),
  });
});
