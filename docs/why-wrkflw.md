# Why wrkflw: market map and differentiation

Status: living strategy note, updated from market research (Sept 2026).
Build order at the bottom. "Never" means never unless the market moves.

## The market has three tiers (they do not substitute)

1. **AI-enhanced team trackers** — Linear (AI Triage/Asks), Notion AI,
   ClickUp, Asana. Built for humans filing tickets; AI summarizes and
   classifies. A task still waits for a person.
2. **Personal schedulers** — Motion, Reclaim, Sunsama. Optimize one
   human's calendar. Irrelevant to agents.
3. **Agent harnesses and boards** — OpenClaw, Devin, Cursor agents,
   Codersera-style boards, Vercel Sandbox. Agents execute; the open
   question is who coordinates them.

wrkflw plays only in tier 3, against OpenClaw first.

## vs Linear

Linear tracks work; wrkflw does work. Assigning a task to an agent *is*
dispatch: a runner picks it up, executes in an isolated worktree, and the
output lands in Review. Linear's AI summarizes threads; it never runs
anything. Different category, no overlap to close.

## vs OpenClaw (the real competitor)

OpenClaw (280k stars, 2026) is a full platform: its own runtime, tools,
skills marketplace, memory, 20+ channels. Our answer is not more
features; it is a different shape:

- **Bridge, not platform.** We never replace the agent. Claude Code,
  Codex, Pi keep their models, tools, logins, and MCP servers. OpenClaw
  asks you to move into its world; we meet you in the terminal you
  already live in.
- **Board, not chat.** OpenClaw is conversation-first with stateless
  follow-through. Every chat, job, and run here becomes a task with
  status, history, and review. Coordination is the product, not a
  side effect of chatting.
- **Park-and-resume asks.** An agent that needs a human at 2pm parks
  (process dies, zero tokens burn) and resumes on reply. Everyone else
  holds sessions open and bills them. This is the single most
  differentiated behavior; sell it first.
- **Blast radius of one.** Run tokens open exactly one task for one run;
  the server never holds repos, model keys, or deploy credentials.
  One harness holding everything is OpenClaw's weakest trust story.
- **App-managed messaging.** Channel config is edited in Settings and
  pulled by the daemon — no JSON-on-a-machine. OpenClaw configures on
  the box.
- **Fix-loop security.** `wrkflw scan` files findings as tasks that an
  agent fixes through claim → worktree → review. Scanners find; we finish.

## Ideas, ranked (research-backed)

**Build now (unique or wedge):**

1. **Attention analytics** — questions-per-run and parked-time per agent
   as a trust score. Nobody sells interruptibility; it decides who runs
   unattended. (Schema already anticipates it.)
2. **In-app two-way chat** — outbox backend exists; add the Inbox reply
   box so phone ↔ board is continuous.
3. **ACP-compatible runner** — OpenClaw dispatches Claude/Codex via its
   Agent Client Protocol. Speaking ACP makes wrkflw runners dispatchable
   *from* OpenClaw: embrace, then absorb.
4. **Windows one-command demo** — done (`scripts/setup-demo.ps1`).
   OpenClaw is Mac/Linux-centric; non-technical Windows onboarding is an
   open wedge.

**Build next:**

5. **Schedules as task fields** — recurring definitions spawning
   occurrences (designed in `docs/agentos-architecture.md` §4).
6. **Runner pairing page** — one-command connect with lease health dots.
7. **Skill export** — one click turns an agent's instructions into a
   ClawHub-compatible skill; meet their marketplace instead of
   fighting it.

**Never (researched, decided):**

8. **WhatsApp via unofficial libs (Baileys/whatsmeow)** — 68% of
   operators report a ban within 12 months; Meta's June 2026 update
   broke QR pairing outright, and generic AI assistants are banned from
   the official API since Jan 2026. Table stakes we deliberately skip
   until a sanctioned path exists.
9. **Model hosting or inference billing** — destroys the cost story
   (your subscriptions, zero markup) and the trust story in one move.
10. **A skills marketplace or memory product** — crowded, and text in
    `instructions` covers 90% of it. Revisit only with demonstrated
    cross-agent reuse.
