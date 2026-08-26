import { test } from "node:test";
import assert from "node:assert/strict";
import os from "node:os";
import path from "node:path";
import { loadConfig, ConfigError } from "./config.js";

const VALID_ENV = { TOGGL_API_TOKEN: "abc123", TOGGL_WORKSPACE_ID: "42" };

const INVALID_CONFIG_CASES = [
  {
    name: "missing TOGGL_API_TOKEN",
    env: { TOGGL_WORKSPACE_ID: "42" },
    expectedPattern: /TOGGL_API_TOKEN/,
  },
  {
    name: "missing TOGGL_WORKSPACE_ID",
    env: { TOGGL_API_TOKEN: "abc123" },
    expectedPattern: /TOGGL_WORKSPACE_ID/,
  },
  {
    name: "non-numeric TOGGL_WORKSPACE_ID",
    env: { TOGGL_API_TOKEN: "abc123", TOGGL_WORKSPACE_ID: "not-a-number" },
    expectedPattern: /TOGGL_WORKSPACE_ID/,
  },
  {
    name: "zero TOGGL_WORKSPACE_ID",
    env: { TOGGL_API_TOKEN: "abc123", TOGGL_WORKSPACE_ID: "0" },
    expectedPattern: /TOGGL_WORKSPACE_ID/,
  },
  {
    name: "negative TOGGL_WORKSPACE_ID",
    env: { TOGGL_API_TOKEN: "abc123", TOGGL_WORKSPACE_ID: "-5" },
    expectedPattern: /TOGGL_WORKSPACE_ID/,
  },
  {
    name: "whitespace-only TOGGL_API_TOKEN",
    env: { TOGGL_API_TOKEN: "   ", TOGGL_WORKSPACE_ID: "42" },
    expectedPattern: /TOGGL_API_TOKEN/,
  },
  {
    name: "whitespace-only TOGGL_WORKSPACE_ID",
    env: { TOGGL_API_TOKEN: "abc123", TOGGL_WORKSPACE_ID: "   " },
    expectedPattern: /TOGGL_WORKSPACE_ID/,
  },
];

for (const caseData of INVALID_CONFIG_CASES) {
  test(`${caseData.name} throws ConfigError matching expected pattern`, () => {
    assert.throws(
      () => loadConfig(caseData.env),
      (err: unknown) => err instanceof ConfigError && caseData.expectedPattern.test((err as Error).message),
    );
  });
}

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

test("whitespace-only TOGGL_CACHE_PATH falls back to the default", () => {
  const env = { ...VALID_ENV, TOGGL_CACHE_PATH: "   " };
  const config = loadConfig(env);
  assert.equal(config.cachePath, path.join(os.homedir(), ".cache", "toggl-mcp", "projects.json"));
});

test("valid env produces a Config with no error thrown", () => {
  const config = loadConfig(VALID_ENV);
  assert.deepEqual(config, {
    togglApiToken: "abc123",
    togglWorkspaceId: 42,
    cachePath: path.join(os.homedir(), ".cache", "toggl-mcp", "projects.json"),
  });
});

test("a ConfigError raised alongside a present TOGGL_API_TOKEN never includes the token's value (TEM-01/02 AC3)", () => {
  const secretToken = "sk-super-secret-toggl-token-value-9f31";
  const env = { TOGGL_API_TOKEN: secretToken, TOGGL_WORKSPACE_ID: "not-a-number" };
  assert.throws(
    () => loadConfig(env),
    (err: unknown) =>
      err instanceof ConfigError &&
      /TOGGL_WORKSPACE_ID/.test((err as Error).message) &&
      !(err as Error).message.includes(secretToken),
  );
});
