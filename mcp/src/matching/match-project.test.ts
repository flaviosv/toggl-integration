import { test } from "node:test";
import assert from "node:assert/strict";
import { extractTicketCode, resolveProject, type CachedProject } from "./match-project.js";

const WORKSPACE_ID = 100;

function project(overrides: Partial<CachedProject>): CachedProject {
  return {
    id: 1,
    name: "Untitled",
    active: true,
    workspaceId: WORKSPACE_ID,
    ...overrides,
  };
}

test("extractTicketCode returns the code for a description with trailing text", () => {
  assert.equal(extractTicketCode("[JSP-10] Creating form"), "JSP");
});

test("extractTicketCode returns the code when the trailing group is empty (Edge Case)", () => {
  assert.equal(extractTicketCode("[JSP-10]"), "JSP");
});

test("extractTicketCode returns null for a non-anchored bracket tag (Edge Case)", () => {
  assert.equal(extractTicketCode("Fixing [JSP-10] today"), null);
});

test("extractTicketCode returns null for a lowercase tag (Edge Case)", () => {
  assert.equal(extractTicketCode("[jsp-10] x"), null);
});

test("extractTicketCode extracts a multi-character alphanumeric code", () => {
  assert.equal(extractTicketCode("[ABC123-42] deploy pipeline"), "ABC123");
});

test("extractTicketCode returns null for a bracket that is not a valid ticket code", () => {
  assert.equal(extractTicketCode("[teachmeto.ai] JSP"), null);
});

test("resolveProject matches case-insensitively after stripping a bracket prefix (TEM-14)", () => {
  const projects = [project({ id: 1, name: "[teachmeto.ai] JSP" })];
  const result = resolveProject("jsp", projects, WORKSPACE_ID);
  assert.deepEqual(result, { status: "matched", project: projects[0] });
});

test("resolveProject matches the whole name when it has no bracket prefix", () => {
  const projects = [project({ id: 1, name: "JSP" })];
  const result = resolveProject("JSP", projects, WORKSPACE_ID);
  assert.deepEqual(result, { status: "matched", project: projects[0] });
});

test("a project named exactly '[teachmeto.ai]' never matches any code (empty stripped remainder, Edge Case)", () => {
  const projects = [project({ id: 1, name: "[teachmeto.ai]" })];
  const result = resolveProject("JSP", projects, WORKSPACE_ID);
  assert.equal(result.status, "no_match");
});

test("resolveProject excludes inactive projects from matching (TEM-14 AC3)", () => {
  const projects = [project({ id: 1, name: "JSP", active: false })];
  const result = resolveProject("JSP", projects, WORKSPACE_ID);
  assert.deepEqual(result, { status: "no_match", extractedCode: "JSP", candidates: [] });
});

test("resolveProject excludes projects from a foreign workspace", () => {
  const projects = [project({ id: 1, name: "JSP", workspaceId: 999 })];
  const result = resolveProject("JSP", projects, WORKSPACE_ID);
  assert.deepEqual(result, { status: "no_match", extractedCode: "JSP", candidates: [] });
});

test("resolveProject returns no_match listing every active in-workspace candidate on zero matches (TEM-15/16)", () => {
  const projects = [
    project({ id: 1, name: "[teachmeto.ai] JSP" }),
    project({ id: 2, name: "[teachmeto.ai] OIQ" }),
    project({ id: 3, name: "Staff Engineer Test" }),
  ];
  const result = resolveProject("XYZ", projects, WORKSPACE_ID);
  assert.deepEqual(result, {
    status: "no_match",
    extractedCode: "XYZ",
    candidates: [
      { id: 1, name: "[teachmeto.ai] JSP" },
      { id: 2, name: "[teachmeto.ai] OIQ" },
      { id: 3, name: "Staff Engineer Test" },
    ],
  });
});

test("resolveProject returns ambiguous listing only the matching projects on multiple matches (TEM-15)", () => {
  const projects = [
    project({ id: 1, name: "[teachmeto.ai] JSP" }),
    project({ id: 2, name: "[teachmeto.ai] OIQ" }),
    project({ id: 3, name: "Staff Engineer Test" }),
    project({ id: 4, name: "[other] jsp" }),
  ];
  const result = resolveProject("JSP", projects, WORKSPACE_ID);
  assert.deepEqual(result, {
    status: "ambiguous",
    extractedCode: "JSP",
    candidates: [
      { id: 1, name: "[teachmeto.ai] JSP" },
      { id: 4, name: "[other] jsp" },
    ],
  });
});

test("resolveProject returns exactly one matched project when only one candidate matches among many", () => {
  const projects = [
    project({ id: 1, name: "[teachmeto.ai] JSP" }),
    project({ id: 2, name: "[teachmeto.ai] OIQ" }),
    project({ id: 3, name: "Staff Engineer Test" }),
  ];
  const result = resolveProject("JSP", projects, WORKSPACE_ID);
  assert.deepEqual(result, { status: "matched", project: projects[0] });
});
