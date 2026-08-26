import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { loadConfig, ConfigError } from "./config.js";
import { TogglClient } from "./toggl/client.js";
import { log } from "./logger.js";
import type { ToolDeps } from "./tools/deps.js";
import { registerListTimeEntries } from "./tools/list-time-entries.js";

async function main(): Promise<void> {
  let config;
  try {
    config = loadConfig(process.env);
  } catch (err) {
    if (err instanceof ConfigError) {
      log("error", "invalid configuration", { reason: err.message });
      process.exit(1);
    }
    throw err;
  }

  const client = new TogglClient({ apiToken: config.togglApiToken });
  const deps: ToolDeps = { client, cachePath: config.cachePath, config };

  const server = new McpServer({ name: "toggl-mcp", version: "0.1.0" });
  registerListTimeEntries(server, deps);

  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main().catch((err) => {
  log("error", "fatal error", { message: err instanceof Error ? err.message : String(err) });
  process.exit(1);
});
