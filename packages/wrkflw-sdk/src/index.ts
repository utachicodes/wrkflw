// Typed client for the wrkflw control-plane API. Works anywhere fetch
// exists (Node 18+, browsers, workers). The token travels as a Bearer
// credential and is redacted from every error surface.

export interface Task {
  id: string;
  title: string;
  status: string;
  priority?: string;
  listName?: string;
  assigneeAgentId?: string;
}

export interface TaskList {
  id: string;
  name: string;
  goal?: string;
  isInbox?: boolean;
}

export interface Agent {
  id: string;
  displayName: string;
  purpose?: string;
}

export interface TaskEntry {
  id: string;
  kind: string;
  authorKind?: string;
  body?: string;
}

export interface InboxMessage {
  id: string;
  taskId: string;
  taskTitle: string;
  kind: string;
  body: string;
  authorName?: string;
}

export interface GatewayConfig {
  channel: string;
  agent: string;
  telegram: { botToken: string; allowUserIds: number[]; allowChatIds: number[] };
  slack: { appToken: string; botToken: string; allowUserIds: string[] };
  imessage: { selfHandles: string[]; allowFrom: string[] };
  primaryDelivery: { channel: string; target: string };
  routes: Array<{ thread: string; agent: string }>;
  lastPulledAt?: string | null;
}

export interface OutboxMessage {
  id: string;
  thread: string;
  body: string;
  createdAt: string;
}

export class WrkflwError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "WrkflwError";
    this.status = status;
    this.code = code;
  }
}

export interface ClientOptions {
  baseUrl?: string;
  token?: string;
  fetch?: typeof fetch;
  timeoutMs?: number;
}

function redact(token: string, text: string): string {
  if (!token) return text;
  return text.split(token).join("[REDACTED]");
}

export function createClient(options: ClientOptions = {}) {
  const baseUrl = (options.baseUrl || "https://wrkflw").replace(/\/+$/, "");
  const token = options.token || "";
  const impl = options.fetch || fetch;
  const timeoutMs = options.timeoutMs || 30000;

  async function request<T>(method: string, path: string, body?: unknown, headers?: Record<string, string>): Promise<T> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    try {
      const response = await impl(baseUrl + path, {
        method,
        signal: controller.signal,
        headers: {
          ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          ...headers,
        },
        body: body !== undefined ? JSON.stringify(body) : undefined,
      });
      const text = await response.text();
      if (!response.ok) {
        let code = "";
        let message = redact(token, text.slice(0, 2000));
        try {
          const payload = JSON.parse(text) as { code?: string; error?: string };
          code = payload.code || "";
          if (payload.error) message = redact(token, payload.error);
        } catch {
          // Keep the raw (redacted) body as the message.
        }
        throw new WrkflwError(response.status, code, message || `request failed with status ${response.status}`);
      }
      if (!text) return undefined as T;
      return JSON.parse(text) as T;
    } catch (error) {
      if (error instanceof WrkflwError) throw error;
      if (error instanceof Error && error.name === "AbortError") {
        throw new WrkflwError(0, "timeout", `request to ${path} timed out after ${timeoutMs}ms`);
      }
      throw error;
    } finally {
      clearTimeout(timer);
    }
  }

  const withIdempotency = (key?: string) =>
    key ? { "Idempotency-Key": key } : undefined;

  return {
    auth: {
      status: () => request<{ authenticated: boolean }>("GET", "/api/v1/me"),
    },
    lists: {
      list: () => request<{ lists: TaskList[] }>("GET", "/api/v1/lists"),
      get: (id: string) => request<TaskList>("GET", `/api/v1/lists/${encodeURIComponent(id)}`),
      create: (input: { name: string; goal?: string }) =>
        request<TaskList>("POST", "/api/v1/lists", input),
    },
    tasks: {
      list: (params: Record<string, string> = {}) => {
        const query = new URLSearchParams(params).toString();
        return request<{ tasks: Task[] }>("GET", `/api/v1/tasks${query ? `?${query}` : ""}`);
      },
      get: (id: string) => request<Task>("GET", `/api/v1/tasks/${encodeURIComponent(id)}`),
      create: (input: {
        title: string;
        description?: string;
        listId?: string;
        parentId?: string;
        date?: string;
        idempotencyKey?: string;
      }) => {
        const { listId, parentId, idempotencyKey, ...rest } = input;
        const path = listId
          ? `/api/v1/lists/${encodeURIComponent(listId)}/tasks`
          : parentId
            ? `/api/v1/tasks/${encodeURIComponent(parentId)}/subtasks`
            : "/api/v1/tasks";
        return request<Task>("POST", path, rest, withIdempotency(idempotencyKey));
      },
      claim: (id: string) =>
        request<Task>("POST", `/api/v1/agent/tasks/${encodeURIComponent(id)}/claim`),
      status: (id: string, status: string) =>
        request<Task>("PATCH", `/api/v1/tasks/${encodeURIComponent(id)}/status`, { status }),
      entries: (id: string) =>
        request<{ entries: TaskEntry[] }>("GET", `/api/v1/tasks/${encodeURIComponent(id)}/entries`),
      comment: (id: string, input: { body: string; idempotencyKey?: string }) =>
        request<TaskEntry>(
          "POST",
          `/api/v1/tasks/${encodeURIComponent(id)}/entries`,
          { kind: "comment", body: input.body },
          withIdempotency(input.idempotencyKey),
        ),
      output: (id: string, input: { body: string; idempotencyKey: string }) =>
        request<TaskEntry>(
          "POST",
          `/api/v1/tasks/${encodeURIComponent(id)}/entries`,
          { kind: "output", body: input.body },
          withIdempotency(input.idempotencyKey),
        ),
    },
    agents: {
      list: () => request<{ agents: Agent[] }>("GET", "/api/v1/agents"),
      get: (id: string) => request<Agent>("GET", `/api/v1/agents/${encodeURIComponent(id)}`),
      create: (input: { displayName: string; purpose?: string }) =>
        request<Agent>("POST", "/api/v1/agents", input),
    },
    inbox: () =>
      request<{ messages: InboxMessage[] }>("GET", "/api/v1/inbox"),
    gateway: {
      getConfig: () => request<GatewayConfig>("GET", "/api/v1/gateway/config"),
      saveConfig: (config: GatewayConfig) =>
        request<GatewayConfig>("PATCH", "/api/v1/gateway/config", config),
      pull: () => request<GatewayConfig>("POST", "/api/v1/gateway/pull"),
      enqueueReply: (input: { thread: string; body: string }) =>
        request<OutboxMessage>("POST", "/api/v1/gateway/outbox", input),
      pollReplies: () =>
        request<{ messages: OutboxMessage[] }>("GET", "/api/v1/gateway/outbox"),
    },
  };
}

export type WrkflwClient = ReturnType<typeof createClient>;
