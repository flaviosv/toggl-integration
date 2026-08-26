import { test } from "node:test";
import assert from "node:assert/strict";
import { toErrorResult } from "./errors.js";
import { TogglApiError, TogglNetworkError } from "./toggl/client.js";

function parse(result: ReturnType<typeof toErrorResult>): unknown {
  assert.equal(result.content.length, 1);
  const item = result.content[0] as { type: string; text: string };
  assert.equal(item.type, "text");
  return JSON.parse(item.text);
}

test("TogglApiError with status 404 maps to toggl_api payload with notFound true", () => {
  const err = new TogglApiError({
    status: 404,
    method: "GET",
    path: "/me/time_entries",
    body: { error: "not found" },
  });
  const result = toErrorResult(err);
  assert.equal(result.isError, true);
  const parsed = parse(result);
  assert.deepEqual(parsed, {
    error: {
      type: "toggl_api",
      status: 404,
      notFound: true,
      method: "GET",
      path: "/me/time_entries",
      body: { error: "not found" },
    },
  });
});

test("TogglApiError with retryAfter carries it through unchanged", () => {
  const err = new TogglApiError({
    status: 429,
    method: "GET",
    path: "/me/projects?include_archived=false",
    body: { error: "rate limited" },
    retryAfter: "30",
  });
  const result = toErrorResult(err);
  assert.equal(result.isError, true);
  const parsed = parse(result);
  assert.deepEqual(parsed, {
    error: {
      type: "toggl_api",
      status: 429,
      retryAfter: "30",
      method: "GET",
      path: "/me/projects?include_archived=false",
      body: { error: "rate limited" },
    },
  });
});

test("TogglApiError without retryAfter leaves the key genuinely absent from the mapped payload", () => {
  const err = new TogglApiError({
    status: 500,
    method: "GET",
    path: "/me/time_entries",
    body: { error: "boom" },
  });
  const result = toErrorResult(err);
  const parsed = parse(result) as { error: Record<string, unknown> };
  assert.equal("retryAfter" in parsed.error, false);
  assert.deepEqual(parsed, {
    error: {
      type: "toggl_api",
      status: 500,
      method: "GET",
      path: "/me/time_entries",
      body: { error: "boom" },
    },
  });
});

test("TogglNetworkError maps to network payload with message and operation", () => {
  const cause = new Error("ECONNRESET");
  const err = new TogglNetworkError("listProjects", cause);
  const result = toErrorResult(err);
  assert.equal(result.isError, true);
  const parsed = parse(result);
  assert.deepEqual(parsed, {
    error: {
      type: "network",
      message: err.message,
      operation: "listProjects",
    },
  });
});

test("every mapped result sets isError true", () => {
  const results = [
    toErrorResult(new TogglApiError({ status: 404, method: "GET", path: "/x", body: null })),
    toErrorResult(new TogglNetworkError("op", new Error("fail"))),
  ];
  for (const result of results) {
    assert.equal(result.isError, true);
  }
});
