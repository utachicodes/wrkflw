import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { MemoryRouter } from "react-router-dom"
import App from "../App"

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const emptyConfig = {
  channel: "",
  agent: "",
  telegram: { botToken: "", allowUserIds: [], allowChatIds: [] },
  slack: { appToken: "", botToken: "", allowUserIds: [] },
  imessage: { selfHandles: [], allowFrom: [] },
  primaryDelivery: { channel: "", target: "" },
  routes: [],
}

function stubApi(config: unknown, requests: string[]) {
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input)
    const method = init?.method || "GET"
    requests.push(`${method} ${path}`)
    if (path === "/api/v1/me") return new Response(JSON.stringify({ authenticated: true, user: { id: "user-1", email: "owner@example.com", displayName: "Owner", theme: "light" } }), { status: 200 })
    if (path === "/api/v1/gateway/config") {
      if (method === "PATCH") return new Response(JSON.stringify(config), { status: 200 })
      return new Response(JSON.stringify(config), { status: 200 })
    }
    if (path === "/api/v1/lists") return new Response(JSON.stringify({ lists: [] }), { status: 200 })
    if (path === "/api/v1/agents") return new Response(JSON.stringify({ agents: [] }), { status: 200 })
    return new Response(JSON.stringify({}), { status: 200 })
  }))
}

function renderMessaging() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={["/app/settings/messaging"]}><App /></MemoryRouter></QueryClientProvider>)
}

test("messaging settings shows the editor, guide link, and pull status", async () => {
  const requests: string[] = []
  stubApi(emptyConfig, requests)
  renderMessaging()

  expect(await screen.findByRole("heading", { name: "Channel" })).toBeInTheDocument()
  expect(screen.getByRole("link", { name: /five-minute setup guide/i })).toHaveAttribute("href", expect.stringContaining("messaging-setup.md"))
  expect(await screen.findByText(/not pulled by any gateway yet/i)).toBeInTheDocument()
  expect(requests).toContain("GET /api/v1/gateway/config")
})

test("typing a bot username generates the Telegram QR link", async () => {
  const requests: string[] = []
  stubApi(emptyConfig, requests)
  renderMessaging()
  await screen.findByRole("heading", { name: "Channel" })

  fireEvent.change(screen.getByLabelText("Bot username"), { target: { value: "my_assistant_bot" } })
  expect(await screen.findByText("https://t.me/my_assistant_bot")).toBeInTheDocument()
})

test("saved config prefills and shows the last pull", async () => {
  const requests: string[] = []
  stubApi({ ...emptyConfig, channel: "telegram", agent: "codex", lastPulledAt: "2026-09-05T20:00:00Z" }, requests)
  renderMessaging()

  await screen.findByText(/last pulled/i)
  expect((screen.getByLabelText("Channel") as HTMLSelectElement).value).toBe("telegram")
})

test("editing and saving sends the full channel config", async () => {
  const requests: string[] = []
  let bodies: string[] = []
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const path = String(input)
    const method = init?.method || "GET"
    requests.push(`${method} ${path}`)
    if (path === "/api/v1/me") return new Response(JSON.stringify({ authenticated: true, user: { id: "user-1", email: "owner@example.com", displayName: "Owner", theme: "light" } }), { status: 200 })
    if (path === "/api/v1/gateway/config" && method === "PATCH") {
      bodies.push(String(init?.body))
      return new Response(JSON.stringify(emptyConfig), { status: 200 })
    }
    if (path === "/api/v1/gateway/config") return new Response(JSON.stringify(emptyConfig), { status: 200 })
    if (path === "/api/v1/lists") return new Response(JSON.stringify({ lists: [] }), { status: 200 })
    if (path === "/api/v1/agents") return new Response(JSON.stringify({ agents: [] }), { status: 200 })
    return new Response(JSON.stringify({}), { status: 200 })
  }))
  renderMessaging()
  await screen.findByLabelText("Channel")

  fireEvent.change(screen.getByLabelText("Channel"), { target: { value: "telegram" } })
  fireEvent.change(screen.getByLabelText("Telegram allowed user IDs"), { target: { value: "123, 456" } })
  fireEvent.click(screen.getByRole("button", { name: "Save channel config" }))

  await waitFor(() => expect(bodies.length).toBe(1))
  const sent = JSON.parse(bodies[0])
  expect(sent.channel).toBe("telegram")
  expect(sent.telegram.allowUserIds).toEqual([123, 456])
  expect(await screen.findByText(/gateway config saved/i)).toBeInTheDocument()
})
