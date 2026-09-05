---
name: frwrd
description: Operate a frwrd personal assistant, inspect its health and jobs, and author or validate frwrd job runbooks. Use when a request concerns frwrd setup, CLI commands, assistant files, scheduled jobs, or delivery behavior.
license: MIT
compatibility: Requires the frwrd CLI and an initialized assistant repository.
metadata:
  frwrd-managed-version: "3"
---

# frwrd

Use frwrd as the gateway around the configured Claude Code, Codex, or Pi
runtime. frwrd owns channels, scheduling, history, security checks, and delivery.
The agent runtime owns reasoning, tools, skill execution, permissions, MCP
servers, and authentication.

## Work in the assistant repository

- Treat `SOUL.md` as user-owned identity and `AGENTS.md` as user-owned repository
  guidance. Do not edit either unless the user asks.
- Read `context/README.md` first when durable user context is relevant. Keep
  useful durable notes under `context/` and evaluation criteria under `evals/`.
- Store complete job runbooks under `jobs/`.
- Treat `skills/frwrd/` and the `frwrd` links under `.agents/skills/` and
  `.claude/skills/` as frwrd-managed. Do not edit them. Codex and Pi share the
  `.agents/skills/` discovery path.
- Keep credentials, configuration containing credentials, sessions, databases,
  audit logs, and job runtime files outside the assistant repository.

## Use the current CLI

Run `frwrd help` when the accepted command forms are unclear. The stable commands
are:

- `frwrd help`
- `frwrd version`
- `frwrd init [path]`
- `frwrd`
- `frwrd doctor`
- `frwrd status`
- `frwrd paths`
- `frwrd reload` or `frwrd restart`
- `frwrd job validate`
- `frwrd job list`
- `frwrd job show <name>`
- `frwrd job run <name>`
- `frwrd job runs [<name>]`
- `frwrd job reviews [<name>]`

All commands accept `--config <path>`, although service status and restart use
the installed service definition. Inspection commands support `--json` where
documented by `frwrd help`: help, version, doctor, status, paths, job validation,
job inspection, runs, and reviews. Commands that start the gateway, change
service state, scaffold files, or run a job reject JSON mode. The first upgraded
inspection that loads configuration and job state records the one-time schedule
migration baseline; follow the release upgrade notes before running it. Prefer
JSON for automation, but still treat paths, handles, and operational metadata
as sensitive. Never expose tokens, message content, or sensitive runtime state
in diagnostics or replies.

## Author jobs safely

1. Read the job format and existing runbooks before editing.
2. Write the complete Markdown runbook directly under `jobs/` when the user
   asks to create or change a job.
3. Keep secrets out of the runbook. Use the configured service or agent
   environment for credentials.
4. Run `frwrd job validate` after every job change. Do not claim success if
   validation fails.
5. Saving a new or changed enabled schedule does not activate it. Tell the user
   that frwrd will present the exact revision for separate owner review.
6. Use `frwrd job show <name>` to inspect the installed result,
   `frwrd job reviews <name>` to inspect schedule activation state, and
   `frwrd job runs <name>` to inspect execution and delivery history.
7. Run `frwrd job run <name>` only when the user asked for the job to execute or
   when execution is a clearly authorized part of the task.

## Reply normally

For an ordinary conversation, return the final reply normally. frwrd sends it
back through the originating channel. Do not invoke the frwrd CLI merely to send
a chat reply.
