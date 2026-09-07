# wrkflw maintainer handoff

Everything a new maintainer needs to run, deploy, and debug this repo.
Product direction lives in `docs/agentos-architecture.md` (target) and
`ARCHITECTURE.md` (current). Market position lives in `docs/why-wrkflw.md`.

## Map

| Part | Code | Ships as |
| --- | --- | --- |
| Control plane | `server/` (Go) + `web/` (React, embedded into the binary) | Cloud Run service + Postgres |
| CLI / runner | `cli/` (Go, stdlib only) | `wrkflw` binary, GitHub releases |
| Gateway | `frwrd/` (Rust) | `frwrd` daemon on the owner's machine |
| Static landing | `index.html`, `styles.css`, `main.js` | Vercel |
| Pipelines | `.github/workflows/` (CI, Gateway, Release CLI), `cloudbuild.yaml`, `scripts/` | GitHub Actions, Cloud Build |

Key flows: session/API-token/agent auth (`server/internal/auth`),
boards/lists/tasks (`server/internal/boards`), agents
(`server/internal/agents`), gateway config + chat outbox
(`server/internal/gateway`, migrations `051`, `052`), messaging UI
(`web/src/pages/other.tsx` → Settings → Messaging), security scan
(`cli/cmd/wrkflw/scan.go`, `docs/security-scan.md`).

## Services and secrets

- Production: Cloud Run (`europe-west1`), Cloud SQL Postgres 18,
  Secret Manager (`DATABASE_URL`, session secret, Resend key, invite code).
- Postgres is the only state. Back up with `pg_dump` (see
  `docs/deploy.md`); restore is tested, not assumed.
- Rotation map: bot tokens → BotFather; API tokens → Settings → API
  access; agent credentials → agent page; passwords → reset flow;
  Postgres superuser → `ALTER USER`. Details in `docs/deploy.md` and
  `docs/messaging-setup.md`.

## Runbooks

- **CI red on `main`**: `gh run list`, then `--log-failed`. Gateway
  (`frwrd`) fails fastest on `fmt` → `clippy` → `build` → `test`;
  all four run locally with the stable Rust toolchain.
- **Required CI red**: recipe order is `build:web`, server suite,
  CLI suite, `test:web`, `test:browser`, installer, cloudbuild. The
  browser file is the usual suspect after landing changes.
- **Database down**: check Cloud SQL status, then `/api/health`
  (pool + capacity). The app fails closed with `503`, never queues.
- **Gateway silent**: `frwrd doctor` on the host; one poller per
  Telegram token; check `lastPulledAt` in Settings → Messaging.
- **Vercel serves stale landing**: `vercel.json` pins the React build;
  hard-refresh first, then check the deployment commit.

## Gotchas

- `serde_json` without `preserve_order` sorts keys; tests assert order.
- React's `muted` JSX attribute does not satisfy autoplay policies;
  the landing sets the property imperatively with a guarded `play()`.
- `printHelp` topics and CLI help text have a test pinning every command.
- Windows dev works, but headless-Chromium browser tests and the Rust
  linker need a full desktop toolchain; Linux CI is the gate.
- Never commit secrets, tokens, or production data. Ever.

## Workflow (house rule)

Branch → PR → review → merge. Direct pushes to `main` are off, even
for small fixes. CI must be green before merge.
