import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { positiveId, rfc3339Timestamp, toEpochMillis, type ToolDeps } from "./schemas.js";
import { getProjects } from "../cache/project-cache.js";
import { extractTicketCode, resolveProject, type CachedProject } from "../matching/match-project.js";
import { toCuratedEntry } from "../time-entries/curate.js";
import { toErrorResult } from "../errors.js";
import { TogglApiError, TogglNetworkError, type RawTimeEntry } from "../toggl/client.js";

const inputSchema = z
  .object({
    id: positiveId,
    description: z.string().optional(),
    start: rfc3339Timestamp.optional(),
    stop: rfc3339Timestamp.optional(),
    project_id: positiveId.optional(),
  })
  .refine(
    (v) => v.description !== undefined || v.start !== undefined || v.stop !== undefined || v.project_id !== undefined,
    { message: "at least one of description, start, stop, project_id must be supplied" },
  );

function validationError(message: string): CallToolResult {
  return {
    content: [{ type: "text", text: JSON.stringify({ error: { type: "validation", message } }) }],
    isError: true,
  };
}

export function registerUpdateTimeEntry(server: McpServer, deps: ToolDeps): void {
  server.registerTool(
    "update_time_entry",
    {
      description:
        "Update a Toggl time entry (read-modify-write). Re-resolves the project only when description changes without an explicit project_id.",
      inputSchema,
    },
    async ({ id, description, start, stop, project_id }): Promise<CallToolResult> => {
      let current: RawTimeEntry;
      try {
        current = await deps.client.getTimeEntry(id);
      } catch (err) {
        return toErrorResult(err as TogglApiError | TogglNetworkError);
      }

      const workspaceId = current.workspace_id ?? deps.config.togglWorkspaceId;

      let projectIdOverride: number | null | undefined;
      let projectsForCuration: CachedProject[] = [];

      if (project_id !== undefined) {
        projectIdOverride = project_id;
      } else if (description !== undefined) {
        const code = extractTicketCode(description);
        if (code === null) {
          projectIdOverride = null;
        } else {
          let projectsResult;
          try {
            projectsResult = await getProjects(deps.client, deps.cachePath);
          } catch (err) {
            return toErrorResult(err as TogglApiError | TogglNetworkError);
          }
          const matchResult = resolveProject(code, projectsResult.projects, workspaceId);
          if (matchResult.status !== "matched") {
            return toErrorResult(matchResult);
          }
          projectIdOverride = matchResult.project.id;
          projectsForCuration = projectsResult.projects;
        }
      }

      const merged: RawTimeEntry = {
        ...current,
        ...(description !== undefined ? { description } : {}),
        ...(start !== undefined ? { start } : {}),
        ...(stop !== undefined ? { stop } : {}),
        ...(projectIdOverride !== undefined ? { project_id: projectIdOverride } : {}),
      };

      const mergedStart = merged.start;
      const mergedStop = merged.stop;
      if (!mergedStart || !mergedStop || toEpochMillis(mergedStop) <= toEpochMillis(mergedStart)) {
        return validationError("the merged stop must be strictly after the merged start");
      }

      if (start !== undefined || stop !== undefined) {
        merged.duration = Math.round((toEpochMillis(mergedStop) - toEpochMillis(mergedStart)) / 1000);
      }

      let updated: RawTimeEntry;
      try {
        updated = await deps.client.updateTimeEntry(workspaceId, id, merged);
      } catch (err) {
        return toErrorResult(err as TogglApiError | TogglNetworkError);
      }

      const projectsById = new Map(projectsForCuration.map((p) => [p.id, p.name]));
      const curated = toCuratedEntry(updated, projectsById);
      return { content: [{ type: "text", text: JSON.stringify(curated) }] };
    },
  );
}
