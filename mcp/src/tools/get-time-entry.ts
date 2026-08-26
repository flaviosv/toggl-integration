import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { positiveId, type ToolDeps } from "./schemas.js";
import { getProjects } from "../cache/project-cache.js";
import { toCuratedEntry } from "../time-entries/curate.js";
import { toErrorResult } from "../errors.js";
import { TogglApiError, TogglNetworkError } from "../toggl/client.js";

const inputSchema = z.object({ id: positiveId });

export function registerGetTimeEntry(server: McpServer, deps: ToolDeps): void {
  server.registerTool(
    "get_time_entry",
    { description: "Get a single Toggl time entry by id, curated.", inputSchema },
    async ({ id }): Promise<CallToolResult> => {
      let entry;
      try {
        entry = await deps.client.getTimeEntry(id);
      } catch (err) {
        return toErrorResult(err as TogglApiError | TogglNetworkError);
      }

      let projectsById = new Map<number, string>();
      let warnings: unknown[] | undefined;
      if (entry.project_id != null) {
        try {
          const result = await getProjects(deps.client, deps.cachePath);
          projectsById = new Map(result.projects.map((p) => [p.id, p.name]));
          if (result.warning) {
            warnings = [result.warning];
          }
        } catch (err) {
          return toErrorResult(err as TogglApiError | TogglNetworkError);
        }
      }

      const curated = toCuratedEntry(entry, projectsById);
      const payload = warnings ? { ...curated, warnings } : curated;
      return { content: [{ type: "text", text: JSON.stringify(payload) }] };
    },
  );
}
