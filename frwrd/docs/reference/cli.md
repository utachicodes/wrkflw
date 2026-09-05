# CLI reference

frwrd has one gateway command, diagnostic and service commands, and a small set
of job commands. All commands accept `--config <path>` anywhere in the argument
list. The default is `$FRWRD_HOME/config.toml`; `FRWRD_HOME` defaults to
`~/.frwrd`. `--config` changes only the selected config file, not the runtime
root.

| Command | Purpose |
| --- | --- |
| `frwrd help`, `frwrd --help` | Print command and option help without loading config or changing files |
| `frwrd version`, `frwrd --version`, `frwrd -V` | Print the installed frwrd version without starting the gateway |
| `frwrd init [path]` | Create and Git-initialize the one assistant repository; defaults to `./assistant` |
| `frwrd` | Start the configured channel gateway and scheduler |
| `frwrd doctor` | Validate config, paths, channel requirements, and required backend binaries |
| `frwrd status` | Show whether the installed launchd or systemd gateway service is running |
| `frwrd paths` | Show the resolved config, assistant, job, and runtime storage paths |
| `frwrd reload`, `frwrd restart` | Restart the managed gateway to load updated config |
| `frwrd job validate` | Validate every installed job; exits non-zero if any are invalid |
| `frwrd job list` | List valid and invalid jobs with backend or error |
| `frwrd job show <name>` | Print the parsed installed job |
| `frwrd job run <name>` | Claim and run one job in the CLI process |
| `frwrd job runs [<name>]` | Print run and delivery history, optionally for one job |
| `frwrd job reviews [<name>]` | Print schedule activation review state and exact revision metadata |

Examples:

```sh
frwrd init ~/Code/assistant
frwrd help
frwrd version
frwrd doctor
frwrd status
frwrd paths
frwrd
frwrd reload
frwrd job validate
frwrd job run repo-review
frwrd job runs repo-review
frwrd job reviews repo-review
```

Unknown commands and missing values fail with the accepted command forms. The
CLI does not currently provide shell completion or separate help pages for
subcommands. A `--help` flag anywhere in the argument list prints the global
help shown by `frwrd --help`.

`frwrd job validate`, `list`, and `show` never activate a schedule. Use
`frwrd job reviews [<name>]` to inspect proposed, approved, rejected,
invalidated, and activated revisions, including their schedules, effective
backend, timeout, work directory, and delivery target. Activation decisions
are made by replying to the durable review question from the exact persisted
allowlisted channel identity that received it.

`frwrd reload` and its `frwrd restart` alias target the service definitions documented by frwrd:
`com.utachicodes.frwrd` under launchd on macOS and the `frwrd.service` user unit
under systemd on Linux. The service definition controls its config path,
environment, and executable; `--config` does not override the service definition
for this command. `frwrd status` reads the same service definition and also
ignores `--config`. Run `frwrd doctor` separately when you want to validate those
settings from the current shell.

`frwrd init` accepts an empty target, the selected config by itself, or a
complete existing assistant layout. It can also safely complete a partial
layout when the selected config already names that exact `assistant_root`.
It refuses unrelated partial non-empty directories, preserves user-owned
`SOUL.md` and `AGENTS.md`, persists one canonical `assistant_root`, and
initializes Git when needed.

Initialization also installs the versioned frwrd capability skill at
`skills/frwrd/` and exposes that one directory through relative links under
`.agents/skills/` for Codex and Pi and `.claude/skills/` for Claude Code.
Repeating `frwrd init` is safe. An unmodified managed copy is refreshed after a
frwrd upgrade; if the skill or an exposure link has diverged, frwrd leaves it
unchanged and reports how to move or restore the conflicting content.
User-created skills and global agent skills are not copied or managed.

## JSON contract

Pass the global `--json` option anywhere in the argument list to select the
version 1 JSON contract. Human-readable output remains the default. JSON mode is
available for:

- `help` and `version`
- `doctor`, `status`, and `paths`
- `job validate`, `job list`, `job show`, `job runs`, and `job reviews`

Commands that start the gateway, change service state, scaffold files, or run a
job reject `--json`. This includes the gateway, `init`, `reload`, `restart`, and
`job run`. Config-loading inspection commands can still migrate the database
schema and capture the one-time upgraded schedule baseline. They do not start a
job or decide a schedule review, but they are not filesystem-state-free. frwrd
does not claim that an interrupted mutation is safe to retry when its outcome
is unknown.

A successful command writes exactly one JSON document and a trailing newline to
stdout. It writes nothing to stderr:

```json
{
  "schema_version": 1,
  "ok": true,
  "command": "paths",
  "data": {}
}
```

A failed command writes exactly one JSON document and a trailing newline to
stderr. It writes nothing to stdout:

```json
{
  "schema_version": 1,
  "ok": false,
  "error": {
    "category": "configuration",
    "message": "configuration not found at ~/.frwrd/config.toml",
    "exit_code": 3,
    "retryable": false
  }
}
```

`error.details` is optional and contains command-specific structured evidence,
such as failed doctor checks or invalid jobs. `retryable` is optional. frwrd
omits it for unexpected failures where it cannot make an honest retry claim.
Diagnostic payloads report whether credentials are configured, never their
values. `job runs` reports content-presence booleans and omits stored result,
evaluation, and error text.

### Exit codes

The category strings and process exit codes are stable within schema version 1:

| Exit code | Category | Meaning |
| --- | --- | --- |
| `0` | success | The command completed |
| `2` | `invalid_input` | Arguments, names, or validated input are invalid |
| `3` | `configuration` | Config or configured local state is missing or invalid |
| `4` | `unavailable_dependency` | A required backend, service manager, or dependency is unavailable |
| `5` | `transient_transport` | A transport failed in a way frwrd knows is safe to retry |
| `6` | `conflict` | Current state conflicts with the requested operation |
| `70` | `unexpected` | frwrd cannot classify the failure safely |

### Command data

Every listed field is required unless marked optional. `integer` values are JSON
integers. Path fields are UTF-8 strings; frwrd replaces invalid filesystem bytes
rather than emitting invalid JSON.

`help` data contains `text` as a string. `version` data contains `name` and
`version` as strings.

`doctor` data:

| Field | Type | Values |
| --- | --- | --- |
| `checks` | array of check objects | All checks in execution order |
| `checks[].name` | string | Stable human-readable check name |
| `checks[].status` | string enum | `pass` or `fail` |
| `checks[].message` | string | Secret-safe explanation |

Failed doctor output places the same object under `error.details`.

`status` data:

| Field | Type | Values |
| --- | --- | --- |
| `manager` | string enum | `launchd` or `systemd` |
| `unit` | string | `com.utachicodes.frwrd` or `frwrd.service` |
| `running` | boolean | `true` only when the normalized state is `active` |
| `state` | string | Normalized service state. Common values are `active`, `inactive`, `not_loaded`, `failed`, `activating`, `deactivating`, or `unknown` |

An operational service-manager failure is an `unavailable_dependency` error,
not a successful inactive status. This command observes the managed process. It
does not open or migrate the configured SQLite database.

`paths` data contains these required string paths:

| Field | Meaning |
| --- | --- |
| `frwrd_home` | Resolved frwrd runtime root |
| `config` | Actually loaded config file, including a `--config` selection |
| `default_config` | Config path derived from `frwrd_home` |
| `assistant_root` | User-owned assistant repository |
| `assistant_context` | Assistant `context` directory |
| `assistant_evals` | Assistant `evals` directory |
| `jobs` | Installed Markdown jobs |
| `jobs_run` | Local run-lock directory |
| `state` | Legacy JSON migration source and retained recovery copy |
| `audit_log` | Structured audit log |
| `database` | Canonical SQLite database for conversations, jobs, delivery, channel cursors, and backend sessions |
| `slack_inbox` | Durable Slack acknowledgement inbox |
| `cache` | frwrd-owned cache directory |
| `imessage_database` | Configured Messages database |

All frwrd-owned fields in this object come from the loaded `FrwrdPaths` owner.
Setting `FRWRD_HOME` relocates its derived fields together. Explicit compatibility
overrides for state, database, audit, and job-run paths appear in their
respective fields without changing `frwrd_home`. The `state` path is not live
runtime state after its one-time import. Live cursors and backend sessions share
the `database` path.

`job validate` and `job list` share catalog data:

| Field | Type | Values |
| --- | --- | --- |
| `valid_count` | integer | Number of entries in `valid` |
| `invalid_count` | integer | Number of entries in `invalid` |
| `valid` | array of valid entry objects | Valid installed jobs |
| `valid[].name` | string | Job slug |
| `valid[].status` | string constant | `valid` |
| `valid[].path` | string | Installed Markdown path |
| `valid[].backend` | string enum | `claude`, `codex`, or `pi` |
| `invalid` | array of invalid entry objects | Invalid installed entries |
| `invalid[].name` | string | Best available filename or job slug |
| `invalid[].status` | string constant | `invalid` |
| `invalid[].path` | string | Rejected entry path |
| `invalid[].message` | string | Validation reason |

`job validate` puts catalog data under `error.details` and exits with
`invalid_input` when `invalid_count` is nonzero. `job list` returns the catalog
successfully so callers can inspect valid and invalid entries together.

`job show` data:

| Field | Type | Values |
| --- | --- | --- |
| `name` | string | Job slug |
| `path` | string | Installed Markdown path |
| `backend` | string enum | `claude`, `codex`, or `pi` |
| `timeout_ms` | integer | Validated timeout in milliseconds |
| `workdir` | string | Resolved backend working directory |
| `snapshot_hash` | string | Validated job snapshot SHA-256 |
| `evals` | array of strings | Assigned eval names |
| `triggers` | array of trigger objects | Validated triggers |
| `triggers[].id` | string | Trigger slug |
| `triggers[].kind` | string constant | `cron` |
| `triggers[].schedule` | string | Five-field cron expression |
| `triggers[].timezone` | string | IANA timezone name |
| `triggers[].enabled` | boolean | Whether the scheduler may enqueue it |
| `body` | string | Runbook instruction body |

`job runs` data:

| Field | Type | Values |
| --- | --- | --- |
| `job_name` | string or null | Requested job filter, or null for all jobs |
| `runs` | array of run objects | Up to 100 newest rows |
| `runs[].id` | string | Run UUID |
| `runs[].job_name` | string | Job slug |
| `runs[].state` | string | Persisted execution state |
| `runs[].backend` | string enum | `claude`, `codex`, or `pi` |
| `runs[].queued_at_ms` | integer | Unix epoch milliseconds |
| `runs[].trigger.kind` | string | `manual` or `cron` |
| `runs[].trigger.id` | string or null | Trigger ID for a scheduled run |
| `runs[].trigger.scheduled_at_ms` | integer or null | Scheduled Unix epoch milliseconds |
| `runs[].execution.has_result` | boolean | Whether stored result text exists |
| `runs[].execution.has_error` | boolean | Whether stored execution error text exists |
| `runs[].evaluation.state` | string | Persisted evaluation state |
| `runs[].evaluation.has_result` | boolean | Whether stored evaluation result text exists |
| `runs[].evaluation.has_error` | boolean | Whether stored evaluation error text exists |
| `runs[].delivery.state` | string | Persisted delivery state |
| `runs[].delivery.attempts` | integer | Delivery attempt count |
| `runs[].delivery.has_error` | boolean | Whether stored delivery error text exists |
| `runs[].delivery.channel` | string or null | Delivery channel |
| `runs[].delivery.target` | string or null | Delivery target |

The run projection queries only `job_runs` from the shared SQLite database. It
does not include co-located channel cursors, backend session IDs, conversation
messages, stored job output, evaluation text, or error text.

`job reviews` data:

| Field | Type | Values |
| --- | --- | --- |
| `job_name` | string or null | Requested job filter, or null for all jobs |
| `reviews` | array of review objects | Up to 100 newest stored schedule review revisions |
| `reviews[].review_id` | string | Exact activation fingerprint |
| `reviews[].job_name` | string | Job slug |
| `reviews[].status` | string enum | `proposed`, `approved`, `rejected`, `invalidated`, or `activated` |
| `reviews[].content_hash` | string | Authored Markdown SHA-256 |
| `reviews[].schedules` | array of trigger objects | Enabled triggers bound to the review |
| `reviews[].schedules[].id` | string | Trigger slug |
| `reviews[].schedules[].kind` | string constant | `cron` |
| `reviews[].schedules[].schedule` | string | Five-field cron expression |
| `reviews[].schedules[].timezone` | string | IANA timezone name |
| `reviews[].schedules[].enabled` | boolean | Always `true` for a reviewed trigger |
| `reviews[].backend` | string enum | Effective `claude`, `codex`, or `pi` backend |
| `reviews[].timeout_ms` | integer | Effective timeout in milliseconds |
| `reviews[].workdir` | string | Resolved backend working directory |
| `reviews[].delivery.channel` | string | Bound delivery channel |
| `reviews[].delivery.target` | string | Bound delivery target |
| `reviews[].reviewed_by` | string or null | Bound actor for a decided revision |
| `reviews[].reason` | string or null | Invalidation or migration reason |

Fields may be added compatibly within version 1. Existing fields, meanings,
category names, and types will not change without a schema-version change.

Shell examples:

```sh
# Read one path.
frwrd paths --json | jq -r '.data.database'

# Fail unless doctor passes, then list failed checks if it does not.
if ! report=$(frwrd doctor --json 2>doctor.json); then
  jq '.error.details.checks[] | select(.status == "fail")' doctor.json
fi

# List valid job names.
frwrd --json job list | jq -r '.data.valid[].name'

# Inspect recent failed run metadata without exposing stored output.
frwrd job runs --json | jq '.data.runs[] | select(.state == "failed")'
```

## Commands sent in chat

These messages are handled by the gateway before backend dispatch:

| Message | Effect |
| --- | --- |
| `/clear`, `/new`, `/reset` | Start a fresh backend session for that conversation |
| `/stop` | Stop the active request; already queued messages continue in order |
| `/help` | Return the available chat commands |

Starting a fresh session preserves canonical history. frwrd can seed the new
backend session with bounded recent turns from the exact channel-qualified
conversation.
