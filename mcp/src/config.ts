import os from "node:os";
import path from "node:path";

export interface Config {
  togglApiToken: string;
  togglWorkspaceId: number;
  cachePath: string;
}

export class ConfigError extends Error {
  constructor(messages: string[]) {
    super(messages.join("; "));
    this.name = "ConfigError";
  }
}

const DEFAULT_CACHE_PATH = path.join(os.homedir(), ".cache", "toggl-mcp", "projects.json");

// TEM-01: collect every missing/invalid variable into one joined ConfigError
// rather than failing on the first, mirroring to-jira's errors.Join pattern.
export function loadConfig(env: NodeJS.ProcessEnv): Config {
  const errors: string[] = [];

  const togglApiToken = env.TOGGL_API_TOKEN;
  if (!togglApiToken || togglApiToken.trim() === "") {
    errors.push("TOGGL_API_TOKEN is required");
  }

  const rawWorkspaceId = env.TOGGL_WORKSPACE_ID;
  let togglWorkspaceId: number | undefined;
  if (!rawWorkspaceId || rawWorkspaceId.trim() === "") {
    errors.push("TOGGL_WORKSPACE_ID is required");
  } else {
    const parsed = Number(rawWorkspaceId);
    if (!Number.isInteger(parsed) || parsed <= 0) {
      errors.push(
        `TOGGL_WORKSPACE_ID must be a positive integer, got ${JSON.stringify(rawWorkspaceId)}`,
      );
    } else {
      togglWorkspaceId = parsed;
    }
  }

  if (errors.length > 0) {
    throw new ConfigError(errors);
  }

  const cachePath = env.TOGGL_CACHE_PATH && env.TOGGL_CACHE_PATH.trim() !== ""
    ? env.TOGGL_CACHE_PATH
    : DEFAULT_CACHE_PATH;

  return {
    togglApiToken: togglApiToken as string,
    togglWorkspaceId: togglWorkspaceId as number,
    cachePath,
  };
}
