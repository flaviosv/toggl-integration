import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { positiveId, rfc3339Timestamp, toEpochMillis, type ToolDeps } from "./schemas.js";
import { getProjects } from "../cache/project-cache.js";
import { extractTicketCode, resolveProject, type CachedProject } from "../matching/match-project.js";
import { toCuratedEntry } from "../time-entries/curate.js";
import { toErrorResult } from "../errors.js";
import { TogglApiError, TogglNetworkError, type CreateTimeEntryBody } from "../toggl/client.js";

const CREATED_WITH = "toggl-mcp";

const inputSchema = z
  .object({
    description: z.string(),
    start: rfc3339Timestamp,
    stop: rfc3339Timestamp,
    project_id: positiveId.optional(),
    workspace_id: positiveId.optional(),
  })
  .refine((v) => toEpochMillis(v.stop) > toEpochMillis(v.start), {
    message: "stop must be strictly after start",
    path: ["stop"],
  });

export function registerCreateTimeEntry(server: McpServer, deps: ToolDeps): void {
  server.registerTool(
    "create_time_entry",
    {
      description:
        "Create a Toggl time entry. Resolves a project from a [TICKET-1] prefix in the description unless project_id is supplied explicitly.",
      inputSchema,
    },
    async ({ description, start, stop, project_id, workspace_id }): Promise<CallToolResult> => {
      const effectiveWorkspaceId = workspace_id ?? deps.config.togglWorkspaceId;

      let resolvedProjectId: number | undefined = project_id;
      let projectsForCuration: CachedProject[] = [];

      if (project_id === undefined) {
        const code = extractTicketCode(description);
        if (code !== null) {
          let projectsResult;
          try {
            projectsResult = await getProjects(deps.client, deps.cachePath);
          } catch (err) {
            return toErrorResult(err as TogglApiError | TogglNetworkError);
          }
          const matchResult = resolveProject(code, projectsResult.projects, effectiveWorkspaceId);
          if (matchResult.status !== "matched") {
            return toErrorResult(matchResult);
          }
          resolvedProjectId = matchResult.project.id;
          projectsForCuration = projectsResult.projects;
        }
      }

      const duration = Math.round((toEpochMillis(stop) - toEpochMillis(start)) / 1000);
      const body: CreateTimeEntryBody = {
        start,
        stop,
        description,
        workspace_id: effectiveWorkspaceId,
        duration,
        created_with: CREATED_WITH,
        ...(resolvedProjectId !== undefined ? { project_id: resolvedProjectId } : {}),
      };

      let raw;
      try {
        raw = await deps.client.createTimeEntry(effectiveWorkspaceId, body);
      } catch (err) {
        return toErrorResult(err as TogglApiError | TogglNetworkError);
      }

      const projectsById = new Map(projectsForCuration.map((p) => [p.id, p.name]));
      const curated = toCuratedEntry(raw, projectsById);
      return { content: [{ type: "text", text: JSON.stringify(curated) }] };
    },
  );
}
