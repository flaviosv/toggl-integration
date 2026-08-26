import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { TogglApiError, TogglNetworkError } from "./toggl/client.js";

export function toErrorResult(err: TogglApiError | TogglNetworkError): CallToolResult {
  const payload = buildErrorPayload(err);
  return {
    content: [{ type: "text", text: JSON.stringify(payload) }],
    isError: true,
  };
}

function buildErrorPayload(err: TogglApiError | TogglNetworkError): { error: Record<string, unknown> } {
  if (err instanceof TogglApiError) {
    if (err.status === 404) {
      return {
        error: {
          type: "toggl_api",
          status: 404,
          notFound: true,
          method: err.method,
          path: err.path,
          body: err.body,
        },
      };
    }
    return {
      error: {
        type: "toggl_api",
        status: err.status,
        method: err.method,
        path: err.path,
        body: err.body,
        ...(err.retryAfter !== undefined ? { retryAfter: err.retryAfter } : {}),
      },
    };
  }

  return {
    error: {
      type: "network",
      message: err.message,
      operation: err.operation,
    },
  };
}
