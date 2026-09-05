# Security scan

`wrkflw scan` screens a codebase for the vulnerability shapes that most
often turn into incidents, then closes the loop: findings become tasks on
the board, an agent fixes them in an isolated worktree, and a person
reviews before anything ships. Scanners find issues; this finishes them.

## Usage

```sh
wrkflw scan [dir] [--min-severity low|medium|high] [--create-tasks]
```

Output is always JSON: the findings, per-severity counts, and how many
tasks were filed. The command exits non-zero when findings at or above
`--min-severity` remain, so `--min-severity high` gates releases in CI
while mediums stay a review queue.

```sh
wrkflw scan ./src --min-severity high
WRKFLW_API_TOKEN=wrkflw_... wrkflw scan ./src --create-tasks
```

## Rules

| Rule | Severity | Catches |
| --- | --- | --- |
| `sql-string-format` | high | SQL text built with Sprintf/format/f-strings/template interpolation |
| `sql-string-concat` | medium | `+` inside a query call; confirm every part is constant or a placeholder |
| `unscoped-data-access` | medium | table access with no visible owner/account scoping |
| `hardcoded-secret` | high | recognizable live credentials (AWS, GitHub, Slack, private keys) |
| `assigned-secret` | medium | literal secrets assigned in code (test fixtures included; verify) |
| `shell-interpolation` | high | interpolated text reaching `os.system`, `shell=True`, `sh -c`, `eval` |
| `unescaped-html-sink` | medium | dynamic content in `dangerouslySetInnerHTML` / `innerHTML` |

Every finding carries a `blastRadius`: the narrowest credential scope
that could exploit it, mapped to wrkflw's trust model (account, machine,
session). Severity is a triage order, not a verdict: constant query
composition and test fixtures are deliberately medium, not high.

## The fix loop

`--create-tasks` files one inbox task per finding with a title of
`[scan:<rule>] <file>:<line>` and the fix guidance in the description.
The idempotency key derives from the finding fingerprint, which hashes
*what* was found rather than the line number, so re-runs never duplicate
tasks even as code shifts.

Recommended flow:

1. Run the scan in CI and file tasks for new findings.
2. Assign the tasks to a security agent.
3. The agent claims each task, works in an isolated worktree, and posts
   the diff as output.
4. A person reviews in the board's Review column. Nothing merges itself.

Skipped paths: `.git`, `node_modules`, `vendor`, `target`, `dist`,
`build`, `__pycache__`, virtualenvs, `.next`, `coverage`, and files
over 2 MiB. Single-line analysis by design: it favors precision on the
most dangerous shapes over whole-program recall.
