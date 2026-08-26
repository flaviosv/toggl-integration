import { z } from "zod";
import fs from "node:fs/promises";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import type { ToolDeps } from "./schemas.js";
import { getProjects } from "../cache/project-cache.js";
import { toErrorResult } from "../errors.js";
import { TogglApiError, TogglNetworkError } from "../toggl/client.js";

const inputSchema = z.object({});

async function readFetchedAt(cachePath: string): Promise<string> {
  try {
    const raw = await fs.readFile(cachePath, "utf8");
    const parsed = JSON.parse(raw) as { fetchedAt?: unknown };
    if (typeof parsed.fetchedAt === "string") {
      return parsed.fetchedAt;
    }
  } catch {
    // fall through to the timestamp fallback below
  }
  return new Date().toISOString();
}

export function registerRefreshProjects(server: McpServer, deps: ToolDeps): void {
  server.registerTool(
    "refresh_projects",
    { description: "Force a refetch of the Toggl project cache, bypassing the 7-day TTL.", inputSchema },
    async (): Promise<CallToolResult> => {
      let result;
      try {
        result = await getProjects(deps.client, deps.cachePath, { forceRefresh: true });
      } catch (err) {
        return toErrorResult(err as TogglApiError | TogglNetworkError);
      }

      const fetchedAt = await readFetchedAt(deps.cachePath);
      const payload: Record<string, unknown> = { count: result.projects.length, fetchedAt };
      if (result.warning) {
        payload.warnings = [result.warning];
      }
      return { content: [{ type: "text", text: JSON.stringify(payload) }] };
    },
  );
}
