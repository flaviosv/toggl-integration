import { test } from "node:test";
import assert from "node:assert/strict";
import { dateOrTimestamp } from "./schemas.js";

test("dateOrTimestamp rejects an invalid calendar date", () => {
  const result = dateOrTimestamp.safeParse("2026-13-40");
  assert.equal(result.success, false);
});

test("dateOrTimestamp accepts a valid RFC3339 timestamp", () => {
  const result = dateOrTimestamp.safeParse("2026-01-01T00:00:00Z");
  assert.equal(result.success, true);
});

test("dateOrTimestamp accepts a valid date-only string", () => {
  const result = dateOrTimestamp.safeParse("2026-01-01");
  assert.equal(result.success, true);
});
