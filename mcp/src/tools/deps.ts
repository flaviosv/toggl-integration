import type { TogglClient } from "../toggl/client.js";
import type { Config } from "../config.js";

export interface ToolDeps {
  client: TogglClient;
  cachePath: string;
  config: Config;
}
