# Security policy

## Supported versions

wrkflw is early software. Security fixes land on the latest release and
`main`. Older releases may not receive fixes.

## Report a vulnerability

Do not open a public issue or pull request for a suspected vulnerability.

Use [GitHub private vulnerability reporting](https://github.com/utachicodes/wrkflw/security/advisories/new)
with the impact and reproduction steps. Strip secrets, tokens, personal
messages, and production data from the report.

The maintainer aims to reply within seven days. Please allow time to
investigate and prepare a fix before any public disclosure.

## What we promise

- Session tokens, API tokens, agent credentials, and reset tokens are
  stored hashed; plaintext is shown once, at creation.
- Agent credentials are scoped to one agent's assigned work; run tokens
  to one task and one run.
- The control plane never holds repos, model keys, or deploy credentials.
- Every read and mutation is scoped to the authenticated account.

For safe operation, read `docs/agentos-architecture.md` (trust section),
`frwrd/docs/security.md`, and `docs/deploy.md` (backup and rotation).
