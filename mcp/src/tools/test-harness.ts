import http from "node:http";
import fsSync from "node:fs";
import os from "node:os";
import path from "node:path";
import type { AddressInfo } from "node:net";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";
import type { ToolDeps } from "./schemas.js";

export interface RecordedRequest {
  method: string;
  url: string;
  headers: http.IncomingHttpHeaders;
  body: string;
}

export interface FakeTogglServer {
  baseUrl: string;
  requests: RecordedRequest[];
  close: () => Promise<void>;
}

export function startFakeToggl(
  handler: (req: http.IncomingMessage, res: http.ServerResponse, body: string) => void,
): Promise<FakeTogglServer> {
  const requests: RecordedRequest[] = [];
  const server = http.createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on("data", (chunk: Buffer) => chunks.push(chunk));
    req.on("end", () => {
      const body = Buffer.concat(chunks).toString("utf8");
      requests.push({ method: req.method ?? "", url: req.url ?? "", headers: req.headers, body });
      handler(req, res, body);
    });
  });
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address() as AddressInfo;
      resolve({
        baseUrl: `http://127.0.0.1:${port}`,
        requests,
        close: () => new Promise((r) => server.close(() => r())),
      });
    });
  });
}

export function sendJson(res: http.ServerResponse, status: number, payload: unknown): void {
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(JSON.stringify(payload));
}

export function makeTmpCachePath(): string {
  const dir = fsSync.mkdtempSync(path.join(os.tmpdir(), "toggl-mcp-tool-test-"));
  return path.join(dir, "projects.json");
}

export function writeProjectCache(
  cachePath: string,
  projects: { id: number; name: string; active: boolean; workspaceId: number }[],
  fetchedAt: string = new Date().toISOString(),
): void {
  fsSync.writeFileSync(cachePath, JSON.stringify({ fetchedAt, projects }), "utf8");
}

export async function connectToolClient(
  registerFns: ((server: McpServer, deps: ToolDeps) => void)[],
  deps: ToolDeps,
): Promise<{ client: Client; close: () => Promise<void> }> {
  const server = new McpServer({ name: "toggl-mcp-test", version: "0.0.0" });
  for (const register of registerFns) {
    register(server, deps);
  }

  const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
  const client = new Client({ name: "toggl-mcp-test-client", version: "0.0.0" });

  await Promise.all([client.connect(clientTransport), server.connect(serverTransport)]);

  return {
    client,
    close: async () => {
      await client.close();
      await server.close();
    },
  };
}

export function parseResult(result: unknown): unknown {
  const content = (result as { content: { type: string; text: string }[] }).content;
  return JSON.parse(content[0].text);
}
