export type LogLevel = "info" | "warn" | "error";

// TEM-24 AC4: stdout is reserved for MCP protocol frames — every log,
// diagnostic, or warning line MUST go to stderr (console.error) only.
export function log(level: LogLevel, message: string, meta?: Record<string, unknown>): void {
  const line: Record<string, unknown> = { level, message };
  if (meta !== undefined) {
    line.meta = meta;
  }
  console.error(JSON.stringify(line));
}
