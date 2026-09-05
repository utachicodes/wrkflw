# wrkflw

One board where people and coding agents share the same work.

You put work on a board. Agents pick it up, do it on machines you own, and message you when they need you. The control plane holds all the state and none of the execution.

## The idea

Coding agents are good at doing work, but there is nowhere to manage that work. You either babysit a chat window while an agent runs, or you hand your repo and credentials to a black box and hope. Task trackers were built for humans filing tickets. Agent harnesses are stateless chat with no memory of what was decided, who is doing what, or what done means.

wrkflw is the missing layer: one board where people and agents share the same work. A task is a unit of work. An agent is config: instructions, a backend, a workspace. A runner is a registered machine. A run is one execution attempt. Agents are an upgrade to a task, not a precondition for the product, so the board is fully usable by a person who never installs anything.

Because runners live on your machines, you bring your own agent runtime (Claude Code, Codex, anything that reads a prompt and exits) and pay for your own tokens on your own subscription. The control plane never holds a repo, a model key, or a deploy credential.

## Who it's for

Builders who already live with coding agents and feel the coordination pain: solo founders running agents overnight, small teams splitting work between people and machines, anyone who wants to delegate from their phone and review on a board. If you have ever thought "I wish I could just text my codebase," that is the gateway. If you have ever thought "I wish someone were keeping track of all these agent runs," that is the control plane.

## Why it's different

- Humans and agents share one inbox. Chat with the gateway, assign work on the board, review runs. It is all the same work.
- No lock-in on execution. Bring your own runtime, tokens, and machines, and switch backends per thread.
- No activation cliff. Useful as a task manager on day one. Agents light up when you are ready.
- Trust boundaries, not vibes. Sandboxes, approval modes, allowlists, and an audit log. Unattended operation is a conscious setup, not an accident.

## Product

wrkflw is one product in three parts:

- **The control plane.** The hosted app. Tasks, agents, runs, the inbox. This is what you sign up for.
- **The runner.** The `wrkflw` CLI, installed separately on your own machine. It polls for work, executes it, and reports back.
- **The gateway.** The `frwrd` daemon, also installed on your machine. It is the always-on messaging and scheduling front end: it answers iMessage, Telegram, and Slack, runs recurring jobs, and feeds the work into the same task inbox.

All three share one assistant repo and one inbox. You never have to install anything; wrkflw is a working task manager on its own, and agents are an upgrade to a task rather than a precondition for the product.

Four primitives: **task**, **agent**, **runner**, **run**. A task is a unit of work. An agent is config: instructions, a backend, a workspace. A runner is a registered machine. A run is one execution attempt.

Because runners live on your machines, you bring your own agent runtime (Claude Code, Codex, anything that reads a prompt and exits) and pay for your own tokens on your own subscription. The control plane never holds a repo, a model key, or a deploy credential.

## Docs

- [Target architecture](docs/agentos-architecture.md)
- [Delivery plan](docs/agentos-plan.md)
- [Current implementation](ARCHITECTURE.md)
- [Install and use the wrkflw CLI](docs/cli.md)
- [Run a coding agent on your wrkflw tasks](docs/watcher.md)
- [Connect messaging in five minutes](docs/messaging-setup.md)
- [Scan code and file fixes as tasks](docs/security-scan.md)

`docs/prd.md` describes the card-first product that came before this and is kept for history only.

## App

- Go server with a React, TypeScript, and shadcn/ui frontend.
- Postgres persistence.
- One board with status columns, an inbox, lists as a scope filter, and task detail.
- One level of subtasks with independent owner and workflow state.
- Named agent identities, assignment, scoped tokens, and CLI execution.
- API tokens for CLI and agent workflows.
- In-repo CLI at `cli/cmd/wrkflw`.
- In-repo Rust gateway at `frwrd/` (the `frwrd` daemon: iMessage, Telegram, and Slack messaging plus a recurring job scheduler).
- Cloud Run and Cloud Build config.

## Local start

```bash
createdb wrkflw_dev
export DATABASE_URL=postgres://localhost/wrkflw_dev?sslmode=disable
export ADMIN_EMAIL=you@example.com
export ADMIN_PASSWORD='use-a-long-password'
just migrate
just seed-admin
npm ci
npm run build:web
just serve
```

Open `http://localhost:8080`.

For local development, run `npm run dev` and open `http://localhost:8081`.
It starts the Go API on port 8080 and Vite on port 8081; Vite proxies API
requests and hot-reloads the React UI as you work. Use `just serve` only when
you need to check the production-style embedded frontend. Override the ports
with `WRKFLW_API_PORT` and `WRKFLW_WEB_PORT` when needed.

## Tests

`just test-unit` builds and type-checks the React frontend, runs its component
tests, and then runs the fast Go and CLI suites without external services. The required
release gate uses PostgreSQL and Chromium:

```bash
createdb wrkflw_test
export WRKFLW_TEST_DATABASE_URL=postgres://localhost/wrkflw_test?sslmode=disable
npm ci
npx playwright install chromium
just test-ci
```

The CI suite fails if a database-backed Go test is skipped. Keep
`WRKFLW_TEST_DATABASE_URL` pointed at a disposable test database because tests
apply migrations and create temporary records.

## CLI

Install the latest release on macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/utachicodes/wrkflw/main/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
export WRKFLW_API_TOKEN=wrkflw_...
wrkflw auth status
wrkflw tasks create --title "Draft launch note"
wrkflw tasks create --list <list-id> --title "Research examples"
wrkflw tasks create --parent <task-id> --title "Human review"
wrkflw tasks pull
wrkflw tasks claim <task-id>
wrkflw tasks status <task-id> needs_review
```

See the full [CLI guide](docs/cli.md) for installation options, upgrades,
authentication, and setup instructions for Claude Code, Codex, and other
shell-based agents. The same guide is published at [wrkflw/cli](https://wrkflw/cli).

## Gateway

The `frwrd` daemon is a separate little binary that runs on your machine
alongside the CLI. It is the always-on messaging front end:

```bash
curl -fsSL https://raw.githubusercontent.com/utachicodes/frwrd/main/install.sh | sh
frwrd doctor
```

It turns Claude Code, Codex, or Pi into an assistant you can message from
iMessage, Telegram, or Slack, and it runs recurring jobs on a schedule. Its
docs live in [`frwrd/docs/`](frwrd/docs/) (also published as its own MkDocs
site at [utachicodes/frwrd](https://github.com/utachicodes/frwrd)).

## Deploy

See [docs/deploy.md](docs/deploy.md).
