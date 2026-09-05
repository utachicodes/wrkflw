# Named Agents and Delegation

## What and why

frwrd should support several named agents inside one assistant. A named agent is
a stable frwrd configuration that selects an agent runtime and an optional
runtime-native profile. For example, `neo` may use Codex while `reviewer` uses
Claude Code.

One named agent may delegate a bounded piece of work to another named agent.
frwrd dispatches the target runtime, records the relationship, and returns the
result to the caller. frwrd does not decide how to solve the task and does not
provide Gmail, MCP, browser, or other domain tools. Each runtime continues to
own its model, tools, skills, permissions, authentication, and external
integrations.

This changes frwrd from selecting one backend into coordinating a small,
user-configured graph of disposable agent runtimes without becoming an agent
runtime itself.

## How it works

The user configures `neo` as the default agent and allows it to delegate to
`reviewer`. An inbound Telegram message is routed to `neo`. frwrd starts Neo's
configured runtime with the normal assistant instructions plus a short list of
the agents Neo may call.

Neo decides that the change needs an independent review. Through its shell tool
it sends the review request on standard input to:

```sh
frwrd agent run reviewer --json
```

The command reads the parent invocation identity from the environment, checks
that Neo may call `reviewer`, claims one delegation worker slot, and records a
child invocation before starting Claude Code. The reviewer receives the common
assistant context, its own runtime profile, and only the review request. It
starts a fresh backend session.

When the reviewer finishes, frwrd stores a bounded result, marks the child
invocation complete, and prints a JSON result to Neo. Neo uses that result to
finish its own answer. Only Neo's final answer is delivered to Telegram. The
reviewer's output remains available in the invocation ledger for inspection.

## Decisions

### Named agents are separate from runtimes

The configured name is the durable identity used by routes, jobs, sessions,
audit events, and delegation. The runtime is one property of that identity.
This permits `neo` and `reviewer` to use different runtimes, or two agents to
use different native profiles in the same runtime. Reusing the current backend
enum as agent identity was rejected because it cannot distinguish two
configurations of Codex.

### Runtime profiles own runtime behavior

frwrd may select an optional runtime-native profile but does not interpret its
contents. Model choice, reasoning effort, tools, MCP servers, permissions,
skills, and authentication stay in the runtime configuration. A generic model
or tool schema in frwrd was rejected because it would couple the gateway to
fast-changing runtime APIs.

### Delegation uses a CLI control surface

The first version uses `frwrd agent run`, called through the runtime's existing
shell tool. This works across coding agents without making frwrd an MCP host or
requiring a new agent protocol. A frwrd-owned MCP delegation server and ACP
client are possible later surfaces over the same internal dispatcher, but are
not required for the first version.

### The caller chooses; frwrd only dispatches

frwrd supplies the allowed agent names and descriptions. The caller decides
whether to delegate and writes the child request. frwrd never classifies the
task, selects a target, rewrites the prompt, combines answers, or retries a
failed child with another runtime.

### Delegated runs are fresh and synchronous

Every delegated run starts a fresh backend session and returns one final reply
to its parent. Persistent worker conversations were rejected for the first
version because they add session ownership, expiry, recovery, and cross-parent
isolation questions. The calling runtime may launch several CLI commands in
parallel when it wants parallel work.

### Capacity rejects instead of waiting

Delegated runs share a cross-process worker limit. A call that cannot claim a
slot fails immediately with `capacity_exhausted`. An internal queue was
rejected because parents wait for children and nested waiting can deadlock when
all worker slots are occupied. The caller may continue without that result or
make a later explicit attempt.

### Delegation policy is operational, not a sandbox

Each agent has an allowlist of targets. The graph must be acyclic and every run
also has a maximum depth. frwrd enforces these rules for correctly propagated
invocation context and audits rejected calls. A runtime running as the same OS
user may still invoke local commands or edit readable configuration, so this
policy does not replace the runtime sandbox or OS permissions.

### No automatic retry or fallback

frwrd does not retry a delegated runtime after failure or timeout because the
runtime may already have made external changes. It also does not silently use
another agent. The parent receives the exact terminal state and decides what
to do next.

## Invariants

1. Every backend execution is attributed to one configured agent name and one
   immutable invocation id.
2. Every delegated invocation has exactly one parent and one root invocation.
3. A delegated invocation starts only when its parent agent allows the target,
   the graph depth is within the configured limit, and a worker slot is held.
4. The configured delegation graph contains no cycles.
5. A delegated invocation never reuses a backend session.
6. Child output is persisted before it is returned to the parent.
7. Child output is never delivered directly to a messaging channel.
8. frwrd never configures or proxies a runtime's domain tools, credentials, or
   external services.
9. A failed, timed-out, or cancelled invocation is never automatically rerun.
10. Changing an agent's runtime identity prevents an old backend session from
    being resumed under the new identity.

## Requirements

- Existing configurations containing only `agent = "claude"`, `"codex"`, or
  `"pi"` continue to work through synthetic named-agent definitions.
- Routes and `jobs_agent` resolve named agents when an `[agents]` catalog is
  present.
- Job frontmatter accepts a named `agent`. Legacy `backend` remains accepted
  during migration, and specifying both is an error.
- Each agent has a non-empty description suitable for showing to an allowed
  delegator.
- frwrd appends delegation instructions only when the active agent has allowed
  targets.
- A delegated run uses the target's configured runtime, native profile,
  timeout, work directory policy, and runtime-owned permissions.
- frwrd applies no unattended permission override. A runtime that requires
  interactive approval must fail or time out unless the user configured that
  runtime profile for non-interactive use.
- `frwrd doctor` checks every referenced runtime binary and reports unsupported
  native profile selection before work starts.
- `frwrd agent runs` shows roots and children without requiring access to a
  runtime's private session store.
- Prompts are accepted through standard input and are never interpolated into a
  shell command by frwrd.
- The service cancels active descendant invocations during parent cancellation
  or shutdown and gives them at most two seconds to exit before forceful
  termination.

## Interfaces and data

Configuration moves from backend names to named agent definitions:

```toml
agent = "neo"
delegation_max_depth = 3
delegation_max_workers = 4
delegation_max_timeout = "30m"

[agents.neo]
runtime = "codex"
runtime_profile = "neo"
description = "Straightforward coding and implementation"
delegates = ["reviewer", "researcher"]
timeout = "20m"

[agents.reviewer]
runtime = "claude"
runtime_profile = "reviewer"
description = "Independent code review"
delegates = []
timeout = "10m"

[agents.researcher]
runtime = "pi"
description = "Focused technical research"
delegates = []
timeout = "10m"
```

`runtime_profile` is optional and opaque to frwrd. An adapter must either map it
to a documented native runtime profile mechanism or reject it during
validation. Agent names are lowercase ASCII slugs containing letters, digits,
and hyphens. Descriptions are trimmed, single-line UTF-8 strings of at most 200
bytes. Timeouts must be positive and no greater than the existing configured
delegation maximum. An omitted agent timeout uses `run_timeout`.

The CLI surface is:

```text
frwrd agent list [--json]
frwrd agent run <agent> [--json]
frwrd agent runs [--root <invocation-id>] [--json]
```

`frwrd agent run` reads one non-empty prompt from standard input. Prompts larger
than 64 KiB are rejected before a runtime starts. With `--json`, success and
failure both produce one JSON object:

```json
{
  "run_id": "018f...",
  "parent_id": "018e...",
  "agent": "reviewer",
  "runtime": "claude",
  "status": "succeeded",
  "reply": "No blocking findings."
}
```

The process exits zero only for `succeeded`. Stable non-success states are
`rejected`, `capacity_exhausted`, `failed`, `timed_out`, and `cancelled`.
Structured failures also include a stable error code such as
`configuration_changed`, `target_not_allowed`, or `output_too_large`.

The SQLite invocation ledger has this logical shape:

```text
agent_invocations
  id
  root_id
  parent_id
  source_kind
  source_id
  agent_name
  runtime
  runtime_profile
  config_hash
  depth
  state
  prompt_hash
  prompt
  result
  error
  created_at
  started_at
  finished_at
```

`source_kind` distinguishes chat, job, evaluator, direct CLI, and delegated
runs. Existing chat and job records reference their invocation ids rather than
duplicating their delivery state. Stored prompts and results are each capped at
64 KiB with an explicit truncation marker. frwrd accepts at most 1 MiB of live
final reply data from a delegated runtime. A larger reply fails with
`output_too_large`; frwrd does not return a partial answer to the parent.

Cross-process capacity uses numbered advisory lock files beneath
`<jobs_run_dir>/delegation-slots/`. A delegated CLI process must hold exactly
one slot lock from before the invocation moves to `running` until after its
terminal state is stored. Failure to claim any slot is
`capacity_exhausted`. Stale lock files are harmless because the operating
system releases advisory locks when a process exits.

Gateway and job backend processes receive the current invocation id, agent
name, config path, config hash, and depth through reserved `FRWRD_` environment
variables. A nested CLI call must load the same catalog hash. If configuration
changed since the parent started, the child is rejected with
`configuration_changed`; a service restart starts new roots on the new catalog.

Backend session state records the agent name and runtime identity. Existing
backend-only state migrates to a synthetic agent with the same name. A change
to an agent's runtime or native profile rotates the session. Description,
timeout, and delegation allowlist edits do not rotate it.

## Naming and identity

Agent names come from `[agents.<name>]` and remain stable until the user renames
the key. Renaming creates a new identity. Old invocation rows remain under the
old name. A route, job, or delegation target that names a missing agent makes
configuration validation fail.

Invocation ids are frwrd-generated UUIDs. The first invocation in a chat turn,
job, evaluator, or direct CLI run is its own root. A child copies the root id
and stores the immediate parent's invocation id. Runtime session ids remain
backend-owned and are never used as invocation ids.

## Failure behavior

Invalid agent names, missing targets, cycles, unsupported profiles, invalid
timeouts, and missing default agents fail configuration validation before the
gateway starts. `frwrd doctor` reports all catalog errors in one pass.

A dispatch rejected by policy or capacity does not start a runtime. It records
a bounded audit event and returns a structured failure to the parent. Runtime
spawn errors, malformed output, missing final replies, non-zero exits, and
timeouts produce one terminal child state. None is retried.

If a parent exits while children are active, frwrd marks those children
`cancelled`, terminates their backend processes, and retains any already
completed child rows. If frwrd crashes, the next command or service start marks
invocations left in `running` as `cancelled` with reason
`interrupted_by_restart`; it never reruns them.

If all configured runtimes are unavailable at startup, validation fails. If one
non-default runtime becomes unavailable later, unrelated agents continue to
run and calls to the unavailable target fail explicitly.

## Limits and budgets

The default maximum delegation depth is three child edges and may be configured
from one to eight. The default cross-process delegated worker limit is four and
may be configured from one to 32. Root chat and job runs do not consume this
delegation limit, but every delegated descendant does.

Each agent timeout is capped by `delegation_max_timeout`. Prompt
and persisted result fields are each limited to 64 KiB, and a live final reply
is limited to 1 MiB. Audit content follows the existing `audit_log_content`
policy. The ledger retains invocation rows under the same local data retention
policy as conversation and job history.

## Acceptance criteria

- A default Codex agent can delegate one task to a Claude agent and incorporate
  the returned result into the single channel reply.
- Two allowed delegated agents can run concurrently, and the ledger records
  both with the same root and parent.
- A fifth concurrent child under the default four-worker limit fails
  immediately with `capacity_exhausted` and starts no runtime.
- Unknown, disallowed, cyclic, self, and over-depth delegation attempts start
  no runtime and return distinct structured errors.
- A delegated agent receives the common assistant instructions, its configured
  native profile, and no parent backend session or hidden transcript.
- Changing an agent from Codex to Claude causes the next chat turn to create a
  fresh Claude session.
- A child success is persisted before its JSON result becomes visible to the
  parent.
- Parent timeout and service shutdown terminate active descendants within two
  seconds and mark them `cancelled`.
- Restart recovery marks interrupted invocations cancelled and never reruns
  them.
- Existing single-backend configurations, routes, jobs, and session state pass
  their current tests without manual migration.
- No implementation code introduces Gmail, external-service OAuth, a generic
  tool registry, an MCP host, or an agent reasoning loop inside frwrd.

## Test approach

Configuration tests cover legacy synthesis, named lookup, graph validation,
profile support, route and job resolution, session identity, and catalog hash
changes.

Fake runner contract tests assert fresh child sessions, instruction separation,
environment propagation, structured output, timeouts, malformed output, and no
permission override. Cross-process tests use fake CLIs and real advisory locks
to prove the worker ceiling and immediate capacity failure.

Gateway tests run a fake coordinator that invokes the CLI against a fake second
runtime. They assert one user-visible reply, correct parent and root ids,
persist-before-return ordering, and isolation between channel delivery and
child output.

Crash and shutdown tests leave nested fake processes active, terminate the
parent or service, and assert bounded cancellation plus restart recovery.
Migration tests load current config and state fixtures and prove unchanged
single-agent behavior.

## Risks

- Shell-based dispatch depends on the parent runtime having shell access. The
  mitigation is to keep the dispatcher behind one internal API so a future ACP
  or MCP surface can reuse it without changing agent identity or ledger data.
- An agent may delegate a poor or dangerous prompt. The target runtime's own
  permissions remain the enforcement boundary, and frwrd records the full
  parent-child relationship for review.
- Nested agents can multiply cost quickly. Depth, target allowlists, timeouts,
  and the global worker ceiling bound the fan-out.
- Killing process trees consistently across macOS and Linux is easy to get
  wrong. Contract tests must use real descendant processes on both platforms.
- Runtime profile flags differ and may change. Each adapter owns its mapping
  and must fail closed when profile selection is unsupported.

## Open questions

- Should a later release add a channel command for temporarily selecting a
  named root agent? This does not block delegation because routes and the
  default agent already select roots.
- Should ACP become the second dispatch surface after the CLI contract is
  stable? This does not block the first version and should not change the
  internal dispatcher.

## Out of scope

- Gmail, calendar, browser, or other external-service integrations in frwrd.
- Model, tool, skill, MCP, credential, or permission configuration shared by
  frwrd.
- frwrd choosing which agent should handle a subtask.
- Persistent delegated-agent conversations.
- Cross-machine or remote agent execution.
- Agent voting, debate, automatic answer merging, or automatic fallback.
- A general workflow language or dependency graph between jobs.
