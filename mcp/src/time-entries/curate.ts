import type { RawTimeEntry } from "../toggl/client.js";

export interface CuratedTimeEntry {
  id: number;
  description: string;
  start: string;
  stop: string | null;
  project: string | null;
}

export function toCuratedEntry(
  entry: RawTimeEntry,
  projectsById: Map<number, string>,
): CuratedTimeEntry {
  const projectId = entry.project_id;
  const project =
    projectId == null
      ? null
      : (entry.project_name ?? projectsById.get(projectId) ?? null);

  return {
    id: entry.id ?? 0,
    description: entry.description ?? "",
    start: entry.start ?? "",
    stop: entry.stop ?? null,
    project,
  };
}
