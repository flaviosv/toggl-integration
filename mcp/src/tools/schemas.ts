import { z } from "zod";
import type { TogglClient } from "../toggl/client.js";
import type { Config } from "../config.js";

const RFC3339_PATTERN = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;
const DATE_ONLY_PATTERN = /^\d{4}-\d{2}-\d{2}$/;

function isValidRfc3339(value: string): boolean {
  return RFC3339_PATTERN.test(value) && !Number.isNaN(Date.parse(value));
}

export const dateOrTimestamp = z.string().refine(
  (value) => DATE_ONLY_PATTERN.test(value) || isValidRfc3339(value),
  { message: "must be a YYYY-MM-DD date or a valid RFC3339 timestamp" },
);

export const positiveId = z.number().int().positive();

export function toEpochMillis(value: string): number {
  return Date.parse(value);
}

export interface ToolDeps {
  client: TogglClient;
  cachePath: string;
  config: Config;
}
