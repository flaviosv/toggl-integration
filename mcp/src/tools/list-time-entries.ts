import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { dateOrTimestamp, positiveId, toEpochMillis } from "./schemas.js";
import type { ToolDeps } from "./deps.js";
import { getProjects } from "../cache/project-cache.js";
import { toCuratedEntry } from "../time-entries/curate.js";
import { toErrorResult } from "../errors.js";
import { TogglApiError, TogglNetworkError } from "../toggl/client.js";
import { log } from "../logger.js";

const inputSchema = z
  .object({
    start_date: dateOrTimestamp,
    end_date: dateOrTimestamp,
    workspace_id: positiveId.optional(),
  })
  .refine((v) => toEpochMillis(v.end_date) >= toEpochMillis(v.start_date), {
    message: "end_date must not be before start_date",
    path: ["end_date"],
  })
  .refine((v) => {
    const startMs = toEpochMillis(v.start_date);
    const endMs = toEpochMillis(v.end_date);
    const spanDays = Math.ceil((endMs - startMs) / (1000 * 60 * 60 * 24));
    return spanDays <= 366;
  }, {
    message: "date range must not exceed 366 days",
    path: ["end_date"],
  });

export function registerListTimeEntries(server: McpServer, deps: ToolDeps): void {
  server.registerTool(
    "list_time_entries",
    {
      description:
        "List Toggl time entries within a date range, curated and filtered to a single workspace.",
      inputSchema,
    },
    async ({ start_date, end_date, workspace_id }): Promise<CallToolResult> => {
      const effectiveWorkspaceId = workspace_id ?? deps.config.togglWorkspaceId;

      let rawEntries;
      try {
        rawEntries = await deps.client.listTimeEntries({ start_date, end_date });
      } catch (err) {
        if (err instanceof TogglApiError || err instanceof TogglNetworkError) {
          log("error", "Toggl request failed", { message: (err as Error).message });
          return toErrorResult(err);
        }
        throw err;
      }

      let hasUnresolvedProject = false;
      const filtered: typeof rawEntries = [];
      for (const entry of rawEntries) {
        if (entry.workspace_id === effectiveWorkspaceId) {
          filtered.push(entry);
          if (entry.project_id != null) {
            hasUnresolvedProject = true;
          }
        }
      }

      let projectsById = new Map<number, string>();
      let projectsWarning: unknown = undefined;
      if (hasUnresolvedProject) {
        try {
          const { projects, warning } = await getProjects(deps.client, deps.cachePath);
          projectsById = new Map(projects.map((p) => [p.id, p.name]));
          projectsWarning = warning;
        } catch (err) {
          if (err instanceof TogglApiError || err instanceof TogglNetworkError) {
            log("error", "Toggl request failed", { message: (err as Error).message });
            return toErrorResult(err);
          }
          throw err;
        }
      }

      const curated = filtered.map((e) => toCuratedEntry(e, projectsById));
      const payload: Record<string, unknown> = { entries: curated };
      if (projectsWarning) payload.warnings = [projectsWarning];
      return { content: [{ type: "text", text: JSON.stringify(payload) }] };
    },
  );
}
