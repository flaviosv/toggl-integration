import { test } from "node:test";
import assert from "node:assert/strict";
import http from "node:http";
import type { AddressInfo } from "node:net";
import { TogglClient, TogglApiError, TogglNetworkError } from "./client.js";

interface RecordedRequest {
  method: string;
  url: string;
  headers: http.IncomingHttpHeaders;
  body: string;
}

interface FakeServer {
  baseUrl: string;
  requests: RecordedRequest[];
  close: () => Promise<void>;
}

function startFakeServer(
  handler: (req: http.IncomingMessage, res: http.ServerResponse, body: string) => void,
): Promise<FakeServer> {
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

function sendJson(res: http.ServerResponse, status: number, payload: unknown): void {
  const text = JSON.stringify(payload);
  res.writeHead(status, { "Content-Type": "application/json" });
  res.end(text);
}

const EXPECTED_AUTH = `Basic ${Buffer.from("tok123:api_token").toString("base64")}`;

test("listTimeEntries issues GET /me/time_entries with query params, correct auth, and resolves parsed array", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 200, [{ id: 1, description: "work" }]);
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    const result = await client.listTimeEntries({ start_date: "2026-01-01", end_date: "2026-01-02" });
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].method, "GET");
    assert.equal(fake.requests[0].url, "/me/time_entries?start_date=2026-01-01&end_date=2026-01-02");
    assert.equal(fake.requests[0].headers.authorization, EXPECTED_AUTH);
    assert.deepEqual(result, [{ id: 1, description: "work" }]);
  } finally {
    await fake.close();
  }
});

test("getTimeEntry issues GET /me/time_entries/{id} with correct auth and resolves parsed object", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 200, { id: 42, description: "entry" });
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    const result = await client.getTimeEntry(42);
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].method, "GET");
    assert.equal(fake.requests[0].url, "/me/time_entries/42");
    assert.equal(fake.requests[0].headers.authorization, EXPECTED_AUTH);
    assert.deepEqual(result, { id: 42, description: "entry" });
  } finally {
    await fake.close();
  }
});

test("createTimeEntry issues POST /workspaces/{wid}/time_entries with body, correct auth, and resolves parsed object", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 200, { id: 7, description: "new entry" });
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    const body = { description: "new entry", start: "2026-01-01T00:00:00Z", workspace_id: 99 };
    const result = await client.createTimeEntry(99, body);
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].method, "POST");
    assert.equal(fake.requests[0].url, "/workspaces/99/time_entries");
    assert.equal(fake.requests[0].headers.authorization, EXPECTED_AUTH);
    assert.deepEqual(JSON.parse(fake.requests[0].body), body);
    assert.deepEqual(result, { id: 7, description: "new entry" });
  } finally {
    await fake.close();
  }
});

test("updateTimeEntry issues PUT /workspaces/{wid}/time_entries/{id} with body, correct auth, and resolves parsed object", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 200, { id: 7, description: "updated" });
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    const body = { id: 7, description: "updated", start: "2026-01-01T00:00:00Z" };
    const result = await client.updateTimeEntry(99, 7, body);
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].method, "PUT");
    assert.equal(fake.requests[0].url, "/workspaces/99/time_entries/7");
    assert.equal(fake.requests[0].headers.authorization, EXPECTED_AUTH);
    assert.deepEqual(JSON.parse(fake.requests[0].body), body);
    assert.deepEqual(result, { id: 7, description: "updated" });
  } finally {
    await fake.close();
  }
});

test("deleteTimeEntry issues DELETE /workspaces/{wid}/time_entries/{id} with correct auth and resolves undefined", async () => {
  const fake = await startFakeServer((_req, res) => {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end();
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    const result = await client.deleteTimeEntry(99, 7);
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].method, "DELETE");
    assert.equal(fake.requests[0].url, "/workspaces/99/time_entries/7");
    assert.equal(fake.requests[0].headers.authorization, EXPECTED_AUTH);
    assert.equal(result, undefined);
  } finally {
    await fake.close();
  }
});

test("listProjects issues GET /me/projects?include_archived=false with correct auth and resolves parsed array", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 200, [{ id: 1, name: "Proj", active: true, workspace_id: 99 }]);
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    const result = await client.listProjects();
    assert.equal(fake.requests.length, 1);
    assert.equal(fake.requests[0].method, "GET");
    assert.equal(fake.requests[0].url, "/me/projects?include_archived=false");
    assert.equal(fake.requests[0].headers.authorization, EXPECTED_AUTH);
    assert.deepEqual(result, [{ id: 1, name: "Proj", active: true, workspace_id: 99 }]);
  } finally {
    await fake.close();
  }
});

test("Authorization header is Basic base64(apiToken:api_token) exactly, for a distinct token", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 200, []);
  });
  try {
    const client = new TogglClient({ apiToken: "different-token-99", baseUrl: fake.baseUrl });
    await client.listProjects();
    const expected = `Basic ${Buffer.from("different-token-99:api_token").toString("base64")}`;
    assert.equal(fake.requests[0].headers.authorization, expected);
  } finally {
    await fake.close();
  }
});

test("404 response throws TogglApiError with status, method, path, and body; exactly one request sent", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 404, { error: "not found" });
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    await assert.rejects(
      () => client.getTimeEntry(999),
      (err: unknown) => {
        assert.ok(err instanceof TogglApiError);
        assert.equal(err.status, 404);
        assert.equal(err.method, "GET");
        assert.equal(err.path, "/me/time_entries/999");
        assert.deepEqual(err.body, { error: "not found" });
        return true;
      },
    );
    assert.equal(fake.requests.length, 1);
  } finally {
    await fake.close();
  }
});

test("other non-2xx (500) response throws TogglApiError with status, method, path, and body", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 500, { error: "boom" });
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    await assert.rejects(
      () => client.listProjects(),
      (err: unknown) => {
        assert.ok(err instanceof TogglApiError);
        assert.equal(err.status, 500);
        assert.equal(err.method, "GET");
        assert.equal(err.path, "/me/projects?include_archived=false");
        assert.deepEqual(err.body, { error: "boom" });
        return true;
      },
    );
  } finally {
    await fake.close();
  }
});

test("404 sets isError-relevant fields but does NOT set retryAfter (absent, not undefined)", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 404, { error: "gone" });
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    await assert.rejects(
      () => client.getTimeEntry(1),
      (err: unknown) => {
        assert.ok(err instanceof TogglApiError);
        assert.equal("retryAfter" in err, false);
        return true;
      },
    );
  } finally {
    await fake.close();
  }
});

test("429 with Retry-After header throws TogglApiError carrying retryAfter", async () => {
  const fake = await startFakeServer((_req, res) => {
    res.writeHead(429, { "Content-Type": "application/json", "Retry-After": "30" });
    res.end(JSON.stringify({ error: "rate limited" }));
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    await assert.rejects(
      () => client.listProjects(),
      (err: unknown) => {
        assert.ok(err instanceof TogglApiError);
        assert.equal(err.status, 429);
        assert.equal(err.retryAfter, "30");
        return true;
      },
    );
  } finally {
    await fake.close();
  }
});

test("429 without Retry-After header leaves retryAfter genuinely absent", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 429, { error: "rate limited" });
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    await assert.rejects(
      () => client.listProjects(),
      (err: unknown) => {
        assert.ok(err instanceof TogglApiError);
        assert.equal(err.status, 429);
        assert.equal("retryAfter" in err, false);
        return true;
      },
    );
  } finally {
    await fake.close();
  }
});

test("network failure (socket destroyed before response) throws TogglNetworkError with operation and cause", async () => {
  const fake = await startFakeServer((req) => {
    req.socket.destroy();
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    await assert.rejects(
      () => client.listProjects(),
      (err: unknown) => {
        assert.ok(err instanceof TogglNetworkError);
        assert.equal(err.operation, "listProjects");
        assert.notEqual(err.cause, undefined);
        return true;
      },
    );
    assert.equal(fake.requests.length, 1);
  } finally {
    await fake.close();
  }
});

test("no retry: a failing call (404) results in exactly one request received by the server", async () => {
  const fake = await startFakeServer((_req, res) => {
    sendJson(res, 404, { error: "not found" });
  });
  try {
    const client = new TogglClient({ apiToken: "tok123", baseUrl: fake.baseUrl });
    await assert.rejects(() => client.deleteTimeEntry(1, 2));
    assert.equal(fake.requests.length, 1);
  } finally {
    await fake.close();
  }
});
