import type { components } from "./generated.js";

export type RawTimeEntry =
  components["schemas"]["github_com_toggl_toggl_api_internal_models.TimeEntry"];
export type RawProject = components["schemas"]["github_com_toggl_toggl_api_internal_models.Project"];

export interface TogglApiErrorOptions {
  status: number;
  method: string;
  path: string;
  body: unknown;
  retryAfter?: string;
}

export class TogglApiError extends Error {
  readonly status: number;
  readonly method: string;
  readonly path: string;
  readonly body: unknown;
  declare readonly retryAfter?: string;  // declare suppresses auto-initialization so retryAfter is genuinely absent (not undefined-valued) when no Retry-After header is present

  constructor(opts: TogglApiErrorOptions) {
    super(`Toggl API error: ${opts.method} ${opts.path} -> ${opts.status}`);
    this.name = "TogglApiError";
    this.status = opts.status;
    this.method = opts.method;
    this.path = opts.path;
    this.body = opts.body;
    if (opts.retryAfter !== undefined) {
      this.retryAfter = opts.retryAfter;
    }
  }
}

export class TogglNetworkError extends Error {
  readonly operation: string;
  readonly cause: unknown;

  constructor(operation: string, cause: unknown) {
    super(`Toggl network error during ${operation}`);
    this.name = "TogglNetworkError";
    this.operation = operation;
    this.cause = cause;
  }
}

const DEFAULT_BASE_URL = "https://api.track.toggl.com/api/v9";

export interface TogglClientOptions {
  apiToken: string;
  baseUrl?: string;
}

export class TogglClient {
  private readonly baseUrl: string;
  private readonly authHeader: string;

  constructor(opts: TogglClientOptions) {
    this.baseUrl = opts.baseUrl ?? DEFAULT_BASE_URL;
    this.authHeader = `Basic ${Buffer.from(`${opts.apiToken}:api_token`).toString("base64")}`;
  }

  private async request<T>(
    path: string,
    operation: string,
  ): Promise<T> {
    let response: Response;
    try {
      response = await fetch(`${this.baseUrl}${path}`, {
        method: "GET",
        headers: {
          Authorization: this.authHeader,
        },
        signal: AbortSignal.timeout(30000),
      });
    } catch (cause) {
      throw new TogglNetworkError(operation, cause);
    }

    const rawBody = await response.text();
    let parsedBody: unknown;
    if (rawBody) {
      try {
        parsedBody = JSON.parse(rawBody);
      } catch {
        parsedBody = rawBody;
      }
    } else {
      parsedBody = undefined;
    }

    if (!response.ok) {
      const retryAfter = response.headers.get("retry-after");
      throw new TogglApiError({
        status: response.status,
        method: "GET",
        path,
        body: parsedBody,
        ...(retryAfter !== null ? { retryAfter } : {}),
      });
    }

    return parsedBody as T;
  }

  listTimeEntries(query: { start_date: string; end_date: string }): Promise<RawTimeEntry[]> {
    const params = new URLSearchParams(query);
    return this.request<RawTimeEntry[]>(
      `/me/time_entries?${params.toString()}`,
      "listTimeEntries",
    );
  }

  listProjects(): Promise<RawProject[]> {
    const params = new URLSearchParams({ include_archived: "false" });
    return this.request<RawProject[]>(`/me/projects?${params.toString()}`, "listProjects");
  }
}
