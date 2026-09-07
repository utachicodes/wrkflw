import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { WrkflwError, createClient } from "./index.js";

interface Recorded {
  method?: string;
  url: string;
  headers: Record<string, string>;
  body?: string;
}

function stubFetch(handler: (call: Recorded) => Response) {
  const calls: Recorded[] = [];
  const fetchImpl = async (url: string, init: RequestInit = {}): Promise<Response> => {
    const headers: Record<string, string> = {};
    for (const [key, value] of new Headers(init.headers).entries()) {
      headers[key] = value;
    }
    calls.push({ method: init.method, url, headers, body: init.body as string | undefined });
    return handler(calls[calls.length - 1]);
  };
  return { calls, fetch: fetchImpl as typeof fetch };
}

const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), { status, headers: { "Content-Type": "application/json" } });

describe("tasks", () => {
  it("lists, claims, and reports output with idempotency keys", async () => {
    const { calls, fetch } = stubFetch((call) => {
      if (call.url.endsWith("/api/v1/tasks?status=queued")) return json({ tasks: [{ id: "t1", title: "Work" }] });
      if (call.url.endsWith("/claim")) return json({ id: "t1", title: "Work", status: "working" });
      if (call.url.endsWith("/entries")) return json({ id: "e1", kind: "output" });
      return json({}, 404);
    });
    const client = createClient({ baseUrl: "https://wrkflw.test", token: "secret", fetch });

    const listed = await client.tasks.list({ status: "queued" });
    assert.equal(listed.tasks[0].id, "t1");
    const claimed = await client.tasks.claim("t1");
    assert.equal(claimed.status, "working");
    const output = await client.tasks.output("t1", { body: "done", idempotencyKey: "k1" });
    assert.equal(output.kind, "output");

    const post = calls.find((call) => call.url.endsWith("/entries"));
    assert.equal(post?.headers["idempotency-key"], "k1");
    for (const call of calls) {
      assert.equal(call.headers["authorization"], "Bearer secret");
    }
  });

  it("creates inbox tasks, list tasks, and subtasks on the right paths", async () => {
    const { calls, fetch } = stubFetch(() => json({ id: "n1" }));
    const client = createClient({ baseUrl: "https://wrkflw.test", token: "t", fetch });

    await client.tasks.create({ title: "Inbox" });
    await client.tasks.create({ title: "Listed", listId: "l1" });
    await client.tasks.create({ title: "Child", parentId: "p1" });

    assert.deepEqual(
      calls.map((call) => `${call.method} ${new URL(call.url).pathname}`),
      ["POST /api/v1/tasks", "POST /api/v1/lists/l1/tasks", "POST /api/v1/tasks/p1/subtasks"],
    );
  });
});

describe("errors", () => {
  it("surfaces status, code, and redacted messages", async () => {
    const { fetch } = stubFetch(() => json({ code: "gone", error: "missing secret-token-here" }, 404));
    const client = createClient({ baseUrl: "https://wrkflw.test", token: "secret-token-here", fetch });

    const error = await client.tasks.get("nope").catch((error: unknown) => error);
    assert.ok(error instanceof WrkflwError);
    assert.equal(error.status, 404);
    assert.equal(error.code, "gone");
    assert.ok(!error.message.includes("secret-token-here"));
  });

  it("times out slow servers", async () => {
    const hanging: typeof fetch = (_url, init) =>
      new Promise((_resolve, reject) => {
        init?.signal?.addEventListener("abort", () => {
          const error = new Error("aborted");
          error.name = "AbortError";
          reject(error);
        });
      });
    const client = createClient({ baseUrl: "https://wrkflw.test", token: "t", fetch: hanging, timeoutMs: 20 });

    const error = await client.inbox().catch((error: unknown) => error);
    assert.ok(error instanceof WrkflwError);
    assert.equal(error.code, "timeout");
  });
});

describe("gateway", () => {
  it("reads config and claims outbox replies", async () => {
    const { calls, fetch } = stubFetch((call) => {
      if (call.url.endsWith("/api/v1/gateway/config")) {
        return json({ channel: "telegram", agent: "codex" });
      }
      return json({ messages: [{ id: "m1", thread: "telegram:dm:1", body: "hi" }] });
    });
    const client = createClient({ baseUrl: "https://wrkflw.test", token: "t", fetch });

    const config = await client.gateway.getConfig();
    assert.equal(config.channel, "telegram");
    const polled = await client.gateway.pollReplies();
    assert.equal(polled.messages[0].thread, "telegram:dm:1");
    assert.ok(calls.some((call) => call.method === "GET" && call.url.endsWith("/api/v1/gateway/outbox")));
  });
});
