import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { positiveId, type ToolDeps } from "./schemas.js";
import { toErrorResult } from "../errors.js";
import { TogglApiError, TogglNetworkError } from "../toggl/client.js";

const inputSchema = z.object({ id: positiveId });

export function registerDeleteTimeEntry(server: McpServer, deps: ToolDeps): void {
  server.registerTool(
    "delete_time_entry",
    { description: "Delete a Toggl time entry by id.", inputSchema },
    async ({ id }): Promise<CallToolResult> => {
      try {
        await deps.client.deleteTimeEntry(deps.config.togglWorkspaceId, id);
      } catch (err) {
        return toErrorResult(err as TogglApiError | TogglNetworkError);
      }

      return { content: [{ type: "text", text: JSON.stringify({ deleted: true, id }) }] };
    },
  );
}
