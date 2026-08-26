import fs from "node:fs/promises";
import path from "node:path";
import type { TogglClient, RawProject } from "../toggl/client.js";
import { log } from "../logger.js";

export interface CachedProject {
  id: number;
  name: string;
  active: boolean;
  workspaceId: number;
}

export interface ProjectCacheFile {
  fetchedAt: string;
  projects: CachedProject[];
}

export interface StaleCacheWarning {
  type: "stale_cache";
  cacheAgeSeconds: number;
  underlyingError: string;
}

export interface GetProjectsResult {
  projects: CachedProject[];
  warning?: StaleCacheWarning;
}

const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000;

function toCachedProject(raw: RawProject): CachedProject {
  return {
    id: raw.id ?? 0,
    name: raw.name ?? "",
    active: raw.active ?? false,
    workspaceId: raw.workspace_id ?? 0,
  };
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

async function readCache(cachePath: string): Promise<ProjectCacheFile | null> {
  try {
    const raw = await fs.readFile(cachePath, "utf8");
    const parsed = JSON.parse(raw) as Partial<ProjectCacheFile>;
    if (typeof parsed.fetchedAt !== "string" || !Array.isArray(parsed.projects)) {
      return null;
    }
    if (Number.isNaN(Date.parse(parsed.fetchedAt))) {
      return null;
    }
    return { fetchedAt: parsed.fetchedAt, projects: parsed.projects };
  } catch {
    return null;
  }
}

async function writeCache(cachePath: string, file: ProjectCacheFile): Promise<void> {
  const tmpPath = `${cachePath}.tmp-${process.pid}`;
  try {
    await fs.mkdir(path.dirname(cachePath), { recursive: true, mode: 0o700 });
    await fs.writeFile(tmpPath, JSON.stringify(file), { encoding: "utf8", mode: 0o600 });
    await fs.rename(tmpPath, cachePath);
  } catch (error) {
    await fs.unlink(tmpPath).catch(() => {});
    log("error", "failed to write project cache", { cachePath, error: errorMessage(error) });
  }
}

/**
 * Fetches projects from Toggl API, using a local cache to minimize requests.
 * The cache is considered fresh for 7 days; after that, a refetch is triggered.
 *
 * @param client - The Toggl API client to use for fetching projects.
 * @param cachePath - The path where the project cache file is stored.
 * @param opts - Optional configuration: `forceRefresh` to ignore cache freshness.
 *
 * @returns An object with `projects` (array of cached projects) and an optional
 *          `warning` (a stale_cache warning if the refetch failed but cached data was used).
 *
 * @throws TogglApiError or TogglNetworkError if the refetch fails and no cache is available.
 */
export async function getProjects(
  client: TogglClient,
  cachePath: string,
  opts?: { forceRefresh?: boolean },
): Promise<GetProjectsResult> {
  const cached = await readCache(cachePath);
  const forceRefresh = opts?.forceRefresh ?? false;
  const stale = cached === null || Date.now() - Date.parse(cached.fetchedAt) >= SEVEN_DAYS_MS;

  if (!stale && !forceRefresh) {
    return { projects: cached.projects };
  }

  let rawProjects: RawProject[];
  try {
    rawProjects = await client.listProjects();
  } catch (error) {
    if (cached !== null) {
      return {
        projects: cached.projects,
        warning: {
          type: "stale_cache",
          cacheAgeSeconds: Math.floor((Date.now() - Date.parse(cached.fetchedAt)) / 1000),
          underlyingError: errorMessage(error),
        },
      };
    }
    throw error;
  }

  const projects = rawProjects
    .filter((p) => {
      if (p.id == null) {
        log("warn", "filtering out project missing id", { project: p });
        return false;
      }
      return true;
    })
    .map(toCachedProject);
  await writeCache(cachePath, { fetchedAt: new Date().toISOString(), projects });
  return { projects };
}
