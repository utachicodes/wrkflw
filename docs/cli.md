# wrkflw CLI

The wrkflw CLI lets you capture, organise, and execute tasks from a terminal. Its
data and workflow commands return JSON, so the same interface works for people,
scripts, and coding agents such as Claude Code and Codex.

## Install

The installer supports macOS and Linux on Intel and ARM. It requires `curl`,
`tar`, and either `sha256sum` or `shasum`.

Install the latest release with one command:

```bash
curl -fsSL https://raw.githubusercontent.com/utachicodes/wrkflw/main/install.sh | sh
```

The installer detects your platform, downloads the matching archive from the
[latest GitHub release](https://github.com/utachicodes/wrkflw/releases/latest),
verifies its SHA-256 checksum, and installs `wrkflw` to `~/.local/bin`.

If that directory is not already on your `PATH`, add it for the current shell:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Add the same line to your shell profile, such as `~/.zshrc` or `~/.bashrc`, to
make it permanent. Then verify the installation:

```bash
wrkflw version
wrkflw help
```

If you prefer to inspect the installer before running it, download it first:

```bash
curl -fsSL https://raw.githubusercontent.com/utachicodes/wrkflw/main/install.sh -o install-wrkflw.sh
less install-wrkflw.sh
sh install-wrkflw.sh
```

### Choose a version or install directory

Pin a release by setting `WRKFLW_VERSION`:

```bash
curl -fsSL https://raw.githubusercontent.com/utachicodes/wrkflw/v1.1.0/install.sh | WRKFLW_VERSION=v1.1.0 sh
```

Choose another install directory with `WRKFLW_INSTALL_DIR`:

```bash
curl -fsSL https://raw.githubusercontent.com/utachicodes/wrkflw/main/install.sh | WRKFLW_INSTALL_DIR=/usr/local/bin sh
```

The second command may need suitable write permission for `/usr/local/bin`.
Windows users can run the Linux installer inside WSL.

## Authenticate

1. Sign in at [wrkflw](https://wrkflw).
2. Open **Settings**.
3. Under **API tokens**, create a token and copy it when it appears.
4. Export it in the terminal where you will run wrkflw or start your agent:

```bash
export WRKFLW_API_TOKEN=wrkflw_...
wrkflw auth status
```

Treat the token like a password. Do not put it in `CLAUDE.md`, `AGENTS.md`,
source code, or a committed `.env` file. An agent started from this terminal
inherits the environment variable.

The CLI connects to `https://wrkflw` by default. For a self-hosted or local
instance, set `WRKFLW_BASE_URL`:

```bash
export WRKFLW_BASE_URL=http://localhost:8080
```

## Basic commands

Create a task without a list to put it in Inbox:

```bash
wrkflw tasks create --title "Draft launch note"
```

Use a list when the bucket is already clear, or a parent when splitting complex work:

```bash
wrkflw tasks create \
  --list <list-id> \
  --title "Draft launch note" \
  --description "Write the first version" \
  --date 2026-07-21

wrkflw tasks create \
  --parent <task-id> \
  --title "Human review"

wrkflw tasks list --list <list-id> --status queued
wrkflw tasks get <task-id>
```

Subtasks are one level deep. They are normal tasks with independent status,
priority, planned date, and agent assignment.

Board and list responses include every active task plus the 20 most recently
updated completed tasks in each list. Task collections omit descriptions to
keep responses small. Use `wrkflw tasks get <task-id>` when you need the full
description.

Page through older completed work with the opaque `nextCursor` returned by the
previous command. Completed history defaults to 20 tasks per page and accepts
up to 100:

```bash
wrkflw tasks list --list <list-id> --status done --limit 20
wrkflw tasks list --list <list-id> --status done --cursor <next-cursor>
```

Priority is optional and crosses lists, so it is how you find urgent work
wherever it lives:

```bash
wrkflw tasks update <task-id> --priority p0
wrkflw tasks list --priority p0
wrkflw tasks update <task-id> --priority ""
```

Run `wrkflw help boards`, `wrkflw help lists`, or `wrkflw help tasks` for every
command and flag.

## Use wrkflw with coding agents

wrkflw is designed for agents that can run shell commands. Start the agent from
the terminal where `WRKFLW_API_TOKEN` is set:

```bash
export WRKFLW_API_TOKEN=wrkflw_...
claude
```

For Codex, use `codex` in place of `claude`. The same setup works with any
agent that can run the `wrkflw` executable and inherit environment variables.

Add the following instructions to your repository's `CLAUDE.md` for Claude
Code or `AGENTS.md` for Codex and other compatible agents. Keep the token out
of the file.

```md
## wrkflw workflow

- Run `wrkflw tasks pull` to find queued work.
- Before starting a task, run `wrkflw tasks claim <task-id>`.
- Only continue when the claim succeeds.
- Read full context with `wrkflw tasks get <task-id>`.
- When work is ready for a person to review, run
  `wrkflw tasks status <task-id> needs_review`.
- After the work is accepted, run `wrkflw tasks status <task-id> done`.
- Treat wrkflw command output as JSON. Preserve IDs exactly.
- Reuse one `--idempotency-key` when retrying a task creation after an
  uncertain result.
```

A typical agent run looks like this:

```bash
wrkflw auth status
wrkflw tasks pull --limit 5
wrkflw tasks claim <task-id>
wrkflw tasks get <task-id>
# The agent performs the work and its checks.
wrkflw tasks status <task-id> needs_review
```

Claiming is atomic. If another agent already claimed the task, the command
fails and the agent should choose another queued task. This prevents two agents
from silently doing the same work.

If an agent polls for new work, poll no faster than once every five seconds,
slow down while idle, and add jitter when several agents start together. A 429
response includes `Retry-After`; wait for that interval before trying again.

### Report through the card

Outside a watcher, `comment` records progress and leaves the card where it is.
Under a watcher, a run-tagged comment observed while the task is still In
Progress is a terminal blocked report: the executor stops and the watcher
retains the worktree. Executors should therefore use managed comments only to
report blockage and then exit. If an output reaches Review before the watcher
observes the comment, the output result takes precedence. `output` records the
completion report and moves the card to Review in the same operation.

Under a watcher, an agent *cannot* set the status itself: while a run owns a
task, wrkflw refuses direct status changes with `managed_run_status_locked`.
Claim reaches In Progress and output reaches Review. Agents started by hand,
without a run, keep using `wrkflw tasks status` as before.

```bash
wrkflw tasks entries <task-id>
wrkflw tasks comment <task-id> --file "${TMPDIR:-/tmp}/note.md" --idempotency-key blocked-1
wrkflw tasks output <task-id> --file "${TMPDIR:-/tmp}/report.md" --idempotency-key output-1
```

Give exactly one of `--body` or `--file`; `--file -` reads standard input. Every
output and every watcher-managed comment needs `--idempotency-key`; a manual
comment outside a managed run may omit it. Reuse the same value to retry after
an uncertain result and no duplicate is created. Write the file outside the
repository so it does not sit in your working tree as an uncommitted change.

### Let a watcher run the agent for you

Everything above assumes you start the agent yourself. `wrkflw watch` does it for
you: it takes tasks assigned to one agent, runs your coding agent on each in an
isolated copy of the repository, and moves finished work to Review.

```bash
wrkflw watch --profile codex
```

See [Run a coding agent on your wrkflw tasks](watcher.md) for setup, what happens
to a task, how to recover a run that did not finish, and the release order.

## Upgrade or uninstall

Run the one-line installer again to replace the current binary with the latest
release. The existing binary is only replaced after the new download passes
checksum verification.

To uninstall the default installation:

```bash
rm "$HOME/.local/bin/wrkflw"
```

If you used `WRKFLW_INSTALL_DIR`, remove `wrkflw` from that directory instead.

## Troubleshooting

- **`wrkflw: command not found`**: Add `~/.local/bin` to `PATH`, then open a new
  terminal.
- **`WRKFLW_API_TOKEN is required`**: Create a token in wrkflw Settings and
  export it before running the command or starting your agent.
- **`unauthorized`**: The token is missing, invalid, or revoked. Create a new
  token and try `wrkflw auth status` again.

For release archives and checksums, see
[wrkflw CLI v1.1.0](https://github.com/utachicodes/wrkflw/releases/tag/v1.1.0)
or the [latest release](https://github.com/utachicodes/wrkflw/releases/latest).
