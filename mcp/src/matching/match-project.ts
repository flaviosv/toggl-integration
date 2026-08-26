// Ticket-code pattern reused verbatim from to-jira/internal/toggl/parse.go
// (`descriptionPattern`), re-expressed as a TS RegExp literal (TEM-13).
const TICKET_CODE_PATTERN = /^\[([A-Z][A-Z0-9]{1,9})-(\d+)\]\s*(.*)$/;
const BRACKET_PREFIX = /^\[[^\]]*\]\s*(.*)$/;

export interface CachedProject {
  id: number;
  name: string;
  active: boolean;
  workspaceId: number;
}

export type MatchResult =
  | { status: "matched"; project: CachedProject }
  | { status: "no_match"; extractedCode: string; candidates: { id: number; name: string }[] }
  | { status: "ambiguous"; extractedCode: string; candidates: { id: number; name: string }[] };

export function extractTicketCode(description: string): string | null {
  const m = TICKET_CODE_PATTERN.exec(description);
  return m ? m[1] : null;
}

function stripBracketPrefix(name: string): string {
  const m = BRACKET_PREFIX.exec(name);
  return m ? m[1] : name;
}

export function resolveProject(
  code: string,
  allProjects: CachedProject[],
  workspaceId: number,
): MatchResult {
  const candidates = allProjects.filter((p) => p.active && p.workspaceId === workspaceId);
  const matches = candidates.filter(
    (p) => stripBracketPrefix(p.name).toLowerCase() === code.toLowerCase(),
  );

  if (matches.length === 1) {
    return { status: "matched", project: matches[0] };
  }
  if (matches.length === 0) {
    return {
      status: "no_match",
      extractedCode: code,
      candidates: candidates.map((p) => ({ id: p.id, name: p.name })),
    };
  }
  return {
    status: "ambiguous",
    extractedCode: code,
    candidates: matches.map((p) => ({ id: p.id, name: p.name })),
  };
}
