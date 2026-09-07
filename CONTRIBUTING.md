# Contributing to wrkflw

Thanks for helping. wrkflw is one board where people and coding agents
share the same work: a Go service with an embedded React frontend, a
Go CLI, and the `frwrd` Rust messaging gateway.

## Setup

```bash
createdb wrkflw_dev
export DATABASE_URL=postgres://localhost/wrkflw_dev?sslmode=disable
export ADMIN_EMAIL=you@example.com
export ADMIN_PASSWORD='use-a-long-password'
just migrate
just seed-admin
npm ci
npm run build:web
just serve   # API + app on :8080; dev UI with `npm run dev` on :8081
```

Windows one-liner for a full demo: `scripts/setup-demo.ps1` (see README).

## Required checks

Run these before opening a pull request. CI runs the same gates.

```bash
just test-unit   # fast: web build+typecheck, unit tests, installer + cloudbuild checks
just test-ci     # full gate: needs Postgres (WRKFLW_TEST_DATABASE_URL) + Chromium
```

Rust (`frwrd/`):

```bash
cd frwrd
cargo fmt --all --check
cargo clippy --locked --all-targets -- -D warnings
cargo build --locked
cargo test --locked
```

Keep changes focused. Add or update tests when behavior changes. Do not
mix formatting-only churn into behavior changes.

## Pull requests

- One change per PR, with the reason and any risk in the description.
- Keep the four primitives (task, agent, runner, run) and six nav items
  intact; see `docs/agentos-architecture.md` before adding concepts.
- Never commit secrets, tokens, or production data. Channel tokens stay
  in gateway config or the API tab, never in fixtures or logs.
- Update the relevant docs (`docs/`, `frwrd/docs/`, README) with the change.

## Bugs and ideas

Use the matching GitHub issue form. Never report a suspected
vulnerability in public; follow [SECURITY.md](SECURITY.md) instead.
