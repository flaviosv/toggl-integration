import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { dateOrTimestamp, positiveId, toEpochMillis, type ToolDeps } from "./schemas.js";
import { getProjects } from "../cache/project-cache.js";
import { toCuratedEntry } from "../time-entries/curate.js";
import { toErrorResult } from "../errors.js";
import { TogglApiError, TogglNetworkError } from "../toggl/client.js";

const inputSchema = z
  .object({
    start_date: dateOrTimestamp,
    end_date: dateOrTimestamp,
    workspace_id: positiveId.optional(),
  })
  .refine((v) => toEpochMillis(v.end_date) >= toEpochMillis(v.start_date), {
    message: "end_date must not be before start_date",
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
        return toErrorResult(err as TogglApiError | TogglNetworkError);
      }

      const filtered = rawEntries.filter((e) => e.workspace_id === effectiveWorkspaceId);

      let projectsById = new Map<number, string>();
      if (filtered.some((e) => e.project_id != null)) {
        try {
          const { projects } = await getProjects(deps.client, deps.cachePath);
          projectsById = new Map(projects.map((p) => [p.id, p.name]));
        } catch (err) {
          return toErrorResult(err as TogglApiError | TogglNetworkError);
        }
      }

      const curated = filtered.map((e) => toCuratedEntry(e, projectsById));
      return { content: [{ type: "text", text: JSON.stringify(curated) }] };
    },
  );
}
