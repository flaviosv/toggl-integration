import { test } from "node:test";
import assert from "node:assert/strict";
import { log } from "./logger.js";

test("log('info', ...) writes exactly one line via console.error", (t) => {
  const errorMock = t.mock.method(console, "error", () => {});
  log("info", "hello");
  assert.equal(errorMock.mock.callCount(), 1);
});

test("log('warn', ...) writes exactly one line via console.error", (t) => {
  const errorMock = t.mock.method(console, "error", () => {});
  log("warn", "careful");
  assert.equal(errorMock.mock.callCount(), 1);
});

test("log('error', ...) writes exactly one line via console.error", (t) => {
  const errorMock = t.mock.method(console, "error", () => {});
  log("error", "boom");
  assert.equal(errorMock.mock.callCount(), 1);
});

test("log never calls console.log for any level (TEM-24 AC4)", (t) => {
  t.mock.method(console, "error", () => {});
  const logMock = t.mock.method(console, "log", () => {});
  log("info", "a");
  log("warn", "b");
  log("error", "c");
  assert.equal(logMock.mock.callCount(), 0);
});

test("log serializes level, message, and meta into the emitted line", (t) => {
  const errorMock = t.mock.method(console, "error", () => {});
  log("error", "toggl request failed", { status: 500 });
  const emitted = errorMock.mock.calls[0].arguments[0] as string;
  const parsed = JSON.parse(emitted);
  assert.deepEqual(parsed, { level: "error", message: "toggl request failed", meta: { status: 500 } });
});

test("log omits meta key from JSON when meta is not provided", (t) => {
  const errorMock = t.mock.method(console, "error", () => {});
  log("info", "hello");
  const emitted = errorMock.mock.calls[0].arguments[0] as string;
  const parsed = JSON.parse(emitted);
  assert.equal("meta" in parsed, false);
});
