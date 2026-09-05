import { agentIDForAssignee, assigneeForTask, availableAgentHandle, buildAssignees, mentionHandle, resolvedAssigneeKey } from "@/lib/assignees"

test.each([
  ["Codex", "codex"],
  ["Research Agent", "research_agent"],
  ["Éditor 2", "editor_2"],
  ["123 Builder", "builder"],
  ["🤖", "agent"],
])("creates a mention-safe handle from %s", (name, expected) => {
  expect(mentionHandle(name, "agent")).toBe(expected)
})

test("builds one human and named agent options with unique handles", () => {
  const options = buildAssignees(
    { id: "owner", email: "abdoullahaljersi@gmail.com", displayName: "Abdoullah Ndao" },
    [
      { id: "codex", displayName: "Codex" },
      { id: "abdoullah-agent", displayName: "Abdoullah" },
    ],
  )

  expect(options.map(option => [option.key, option.handle])).toEqual([
    ["human", "abdoullah"],
    ["agent:codex", "codex"],
    ["agent:abdoullah-agent", "abdoullah_2"],
  ])
  expect(assigneeForTask({ assigneeAgentId: "codex" }, options).handle).toBe("codex")
  expect(assigneeForTask({ assigneeAgentId: "" }, options).handle).toBe("abdoullah")
  expect(agentIDForAssignee("human")).toBe("")
  expect(agentIDForAssignee("agent:codex")).toBe("codex")
  expect(availableAgentHandle("Abdoullah", options)).toBe("abdoullah_3")
  expect(availableAgentHandle("Abdoullah", options, "agent:abdoullah-agent")).toBe("abdoullah_2")
  expect(resolvedAssigneeKey("agent:missing", options)).toBeUndefined()
  expect(assigneeForTask({ assigneeAgentId: "missing" }, options)).toMatchObject({ kind: "agent", handle: "unavailable_agent" })
})
