import { test } from "node:test";
import assert from "node:assert/strict";
import { toCuratedEntry } from "./curate.js";
import type { RawTimeEntry } from "../toggl/client.js";

const BASE_ENTRY: RawTimeEntry = {
  id: 1,
  description: "[TOGGL-2] Building the client",
  start: "2026-08-26T09:00:00Z",
  stop: "2026-08-26T10:00:00Z",
  project_id: 42,
};

test("entry with a project_id resolved in projectsById maps to the resolved project name", () => {
  const projectsById = new Map([[42, "Time Entries MCP"]]);
  const result = toCuratedEntry(BASE_ENTRY, projectsById);
  assert.deepEqual(result, {
    id: 1,
    description: "[TOGGL-2] Building the client",
    start: "2026-08-26T09:00:00Z",
    stop: "2026-08-26T10:00:00Z",
    project: "Time Entries MCP",
  });
});

test("entry with project_id null maps to project: null", () => {
  const entry: RawTimeEntry = { ...BASE_ENTRY, project_id: null };
  const projectsById = new Map([[42, "Time Entries MCP"]]);
  const result = toCuratedEntry(entry, projectsById);
  assert.equal(result.project, null);
});

test("entry with project_id absent maps to project: null", () => {
  const { project_id, ...rest } = BASE_ENTRY;
  void project_id;
  const entry: RawTimeEntry = rest;
  const projectsById = new Map([[42, "Time Entries MCP"]]);
  const result = toCuratedEntry(entry, projectsById);
  assert.equal(result.project, null);
});

test("entry with a project_id not present in projectsById (stale/mismatched cache) degrades to project: null without throwing", () => {
  const projectsById = new Map([[999, "Some Other Project"]]);
  const result = toCuratedEntry(BASE_ENTRY, projectsById);
  assert.equal(result.project, null);
});

test("output carries exactly the five CuratedTimeEntry fields, no extra raw fields leak through", () => {
  const entryWithExtras = {
    ...BASE_ENTRY,
    workspace_id: 99,
    tags: ["a", "b"],
    billable: true,
  } as RawTimeEntry;
  const projectsById = new Map([[42, "Time Entries MCP"]]);
  const result = toCuratedEntry(entryWithExtras, projectsById);
  assert.deepEqual(Object.keys(result).sort(), ["description", "id", "project", "start", "stop"]);
});
