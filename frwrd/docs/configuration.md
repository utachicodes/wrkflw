# Configuration

frwrd owns one runtime root. `FRWRD_HOME` selects it and defaults to `~/.frwrd`.
The default config is `$FRWRD_HOME/config.toml`. Pass `--config <path>` to use a
different file for a gateway, doctor, init, or job command.

```sh
frwrd doctor
frwrd
```

Paths beginning with `~` are expanded. Invalid values, unknown fields inside
provider sections, unsafe path overlap, and removed gateway permission settings
fail configuration load with an actionable error.

The path precedence is:

1. `--config` selects the config file when present.
2. `FRWRD_HOME` selects the runtime root, even when `--config` selects a file elsewhere.
3. Explicit `state_path`, `database_path`, `audit_log_path`, and
   `jobs_run_dir` settings override their matching derived locations.
4. Without `FRWRD_HOME`, frwrd derives the root from `HOME` as `~/.frwrd`.

If neither `FRWRD_HOME` nor `HOME` is available, commands that need
configuration fail with an instruction to set `FRWRD_HOME`. Help and version
commands still work. Use distinct `FRWRD_HOME` values to run isolated frwrd
installations. `assistant_root` is never derived from `FRWRD_HOME`; keep the
assistant repository outside the runtime root.

Create the one assistant repository and persist its root before editing the
rest of the config:

```sh
frwrd init ~/Code/assistant
```

For a new file, init writes a private, owner-only Telegram and Codex starting
point with empty `telegram.bot_token` and `telegram.allow_user_ids` values.
Fill both in before running frwrd. frwrd derives `SOUL.md`, `context/`, `evals/`,
and `jobs/` from `assistant_root`. At run time it composes separate frwrd policy,
user-owned identity, and resolved-path system sections in memory. It does not
write machine paths into the repository.

Root configuration, route, and primary-delivery tables do not
yet reject every unknown key. Use the documented names, then run `frwrd doctor`;
do not assume a silent key changed runtime behavior.

## Minimal configuration

```toml
channel = "telegram"
agent = "codex"
assistant_root = "~/Code/assistant"

[telegram]
bot_token = "token-from-BotFather"
allow_user_ids = [123456789]
```

`channel` is the easiest single-provider setup. `agent` is `claude`, `codex`,
or `pi`. frwrd preserves backend permission settings for chats. Codex and Claude
jobs bypass interactive permissions so unattended runs can complete.

### Pi setup

Install Pi from [pi.dev](https://pi.dev/) and configure a model provider or
complete its authentication as the same user that runs frwrd. Confirm `pi
--version` works in the service environment, then select it:

```toml
agent = "pi"
```

frwrd finds `pi` through `PATH`, runs `pi --print --mode json`, and stores the
session ID from Pi's JSON event stream. It resumes the session with `--session`.
Clearing a conversation discards that
mapping, so the next turn creates a fresh Pi session. frwrd passes the composed
policy, identity, and path sections as system instructions, separate from the
untrusted fresh message context. Pi is not required unless the default backend,
an enabled route, or `jobs_agent` selects it.

## Channels

### iMessage

```toml
[imessage]
db_path = "~/Library/Messages/chat.db"
self_handles = ["you@icloud.com"]
allow_from = ["+15551234567"]
```

At least one `self_handles` or `allow_from` value is required when iMessage is
enabled. See the [iMessage guide](channels/imessage.md).

### Telegram

```toml
[telegram]
bot_token = "token-from-BotFather"
allow_user_ids = [123456789]
allow_chat_ids = []
```

At least one stable numeric user or chat ID is required. Keep the config file
private. `frwrd init` creates new config files with mode `0600` on Unix. Set
`TELEGRAM_BOT_TOKEN` when an environment variable is a better fit. See the
[Telegram guide](telegram.md).

### Slack

```toml
[slack]
allow_user_ids = ["U012ABCDEF"]
```

Slack requires both `SLACK_APP_TOKEN` and `SLACK_BOT_TOKEN`, or the matching
`slack.app_token` and `slack.bot_token` values in the private config. At least
one exact Slack member ID is required. See the [Slack guide](slack.md).

Telegram voice notes are optional. Configure the shared voice provider with:

```toml
[voice]
openai_api_key = "your-api-key"
name = "cedar"
```

`OPENAI_API_KEY` remains available as a higher-priority override for CI and
service secret injection. `voice.name` is optional and defaults to `cedar`.
Supported names are `alloy`, `ash`, `ballad`, `coral`, `echo`, `fable`, `nova`,
`onyx`, `sage`, `shimmer`, `verse`, `marin`, and `cedar`. Without either API
key value, text remains fully available and voice notes get a helpful fallback.
See [Voice Messages](telegram.md#voice-messages).

### Run both providers

Use `channels` instead of `channel`:

```toml
channels = ["imessage", "telegram", "slack"]
agent = "codex"

[imessage]
self_handles = ["you@icloud.com"]

[telegram]
bot_token = "token-from-BotFather"
allow_user_ids = [123456789]

[primary_delivery]
channel = "telegram"
target = "123456789"
```

Each provider polls independently, keeps its own cursor, and replies through
the channel and exact conversation that originated the message. Failure in one
provider does not stop the other.

`primary_delivery` is the destination for scheduled job results. The channel
must be enabled and the target must appear in that channel's allowlist.
Telegram topic targets use `"<chat-id>:<topic-id>"`.
Slack primary targets use an allowlisted member ID such as `U012ABCDEF`.

## Routing

Routes can override the backend for a channel or exact thread:

```toml
[[routes]]
channel = "telegram"
agent = "codex"

[[routes]]
thread = "telegram:dm:123456789"
agent = "claude"
```

Precedence is:

1. exact thread or topic route
2. parent Telegram private-chat route for a topic
3. channel route
4. root `agent`

Thread keys are:

- `imessage:self:<handle>`
- `imessage:dm:<handle>`
- `telegram:dm:<chat-id>`
- `telegram:dm:<chat-id>:topic:<topic-id>`
- `slack:dm:<workspace-id>:<dm-channel-id>`

## Agent permissions

For chats, frwrd invokes Claude Code, Codex, and Pi without overriding their
sandbox, approval mode, or tool lists. Codex and Claude jobs bypass interactive
permissions because scheduled work has no operator available to approve
requests. Review [permissions and security](security.md) before enabling jobs.

## Settings reference

### Core

| Setting | Default | Purpose |
| --- | --- | --- |
| `channel` | `"imessage"` | Single enabled provider when `channels` is empty |
| `channels` | `[]` | Concurrent enabled providers |
| `agent` | `"claude"` | Default backend |
| `poll_interval` | `"3s"` | Delay between channel polls |
| `run_timeout` | `"10m"` | Maximum chat backend run time |

### iMessage

| Setting | Default | Purpose |
| --- | --- | --- |
| `imessage.db_path` | `~/Library/Messages/chat.db` | Messages database read by the macOS channel |
| `imessage.self_handles` | `[]` | Own handles accepted in one-to-one self chats |
| `imessage.allow_from` | `[]` | Other trusted one-to-one sender handles |

### Telegram

| Setting | Default | Purpose |
| --- | --- | --- |
| `telegram.bot_token` | `TELEGRAM_BOT_TOKEN` fallback | Private Bot API token; the environment value is used when this is omitted |
| `telegram.allow_user_ids` | `[]` | Trusted numeric sender IDs |
| `telegram.allow_chat_ids` | `[]` | Trusted numeric private-chat IDs |

### Slack

| Setting | Default | Purpose |
| --- | --- | --- |
| `slack.app_token` | `SLACK_APP_TOKEN` fallback | App-level Socket Mode token with `connections:write` |
| `slack.bot_token` | `SLACK_BOT_TOKEN` fallback | Bot token used for `auth.test`, replies, and progress |
| `slack.allow_user_ids` | `[]` | Trusted stable Slack member IDs |

### Voice

| Setting | Default | Purpose |
| --- | --- | --- |
| `voice.openai_api_key` | `OPENAI_API_KEY` fallback | Optional key for Telegram transcription and spoken replies; the environment value has priority |
| `voice.name` | `"cedar"` | OpenAI voice used for spoken replies |

### wrkflw control plane

The `[wrkflw]` table connects frwrd to your wrkflw control plane. When
`mirror = true`, frwrd keeps one wrkflw inbox task per conversation thread: the
first inbound message creates the task, later messages and every assistant
reply become task entries. Mirroring is best effort and never blocks the
channel loop.

| Setting | Default | Purpose |
| --- | --- | --- |
| `wrkflw.base_url` | `WRKFLW_BASE_URL` fallback, then `https://wrkflw` | Control-plane API base URL |
| `wrkflw.token` | `WRKFLW_API_TOKEN` fallback | Personal API token used to create tasks and entries |
| `wrkflw.mirror` | `false` | Mirror inbound messages and replies as inbox-task entries |

```toml
[wrkflw]
base_url = "https://wrkflw"
token = "replace-with-your-wrkflw-api-token"
mirror = true
```

Prefer `WRKFLW_API_TOKEN` over an inline token, and keep this file private.

### Delivery and routes

| Setting | Default | Purpose |
| --- | --- | --- |
| `primary_delivery.channel` | none | Enabled channel used for scheduled job results |
| `primary_delivery.target` | none | Allowlisted destination on the primary channel |
| `routes[].thread` | none | Exact channel-qualified thread key to match |
| `routes[].channel` | none | Enabled provider to match when no exact thread route wins |
| `routes[].agent` | required per route | Backend selected by the matching route |

### Local state

| Setting | Default | Purpose |
| --- | --- | --- |
| `assistant_root` | required for new setups | Canonical root of the one assistant repository; `SOUL.md`, `context/`, `evals/`, and `jobs/` are derived |
| `state_path` | `$FRWRD_HOME/state.json` | Legacy JSON source retained for one-time cursor and session migration |
| `database_path` | `$FRWRD_HOME/frwrd.db` | Canonical history, jobs, delivery, channel cursors, and backend sessions |
| `audit_log_path` | `$FRWRD_HOME/audit.jsonl` | Structured local audit log |
| `audit_log_content` | `false` | Include message and reply content in audit events |

### Jobs

| Setting | Default | Purpose |
| --- | --- | --- |
| `jobs_agent` | root `agent` | Default jobs backend |
| `jobs_max_timeout` | `"30m"` | Maximum accepted job timeout |
| `jobs_run_dir` | `$FRWRD_HOME/run` | Local advisory locks |
| `jobs_max_workers` | `2` | Concurrent scheduled job workers |

frwrd validates that runtime state, locks, external config files, and job work
directories do not overlap in unsafe ways.
Runtime state and secrets must stay outside the Git-versioned assistant
repository.

frwrd also derives the Slack recovery inbox as
`$FRWRD_HOME/state.json.slack-inbox.db` and uses `$FRWRD_HOME/cache` for
disposable agent handoff files. When `state_path` is explicitly set, the Slack
inbox stays beside it as `<state_path>.slack-inbox.db` to preserve existing
installations.

## Complete example

```toml
channels = ["imessage", "telegram"]
agent = "codex"
assistant_root = "~/Code/assistant"
poll_interval = "3s"
run_timeout = "10m"

[imessage]
self_handles = ["you@icloud.com"]
allow_from = []

[telegram]
bot_token = "token-from-BotFather"
allow_user_ids = [123456789]

[primary_delivery]
channel = "telegram"
target = "123456789"

[[routes]]
thread = "telegram:dm:123456789"
agent = "claude"
```

Legacy flat channel fields remain accepted for migration, but new
configurations should use `[imessage]` and `[telegram]`. JSON configuration and
gateway permission fields are no longer supported. Configure permissions in
the selected agent instead.

frwrd resolves `claude`, `codex`, and `pi` through the service `PATH`. Configure
the backend's model in that backend rather than in frwrd. Telegram environment
tokens always use `TELEGRAM_BOT_TOKEN`; the iMessage reply marker is internal.

Legacy `assistant_dir` and `jobs_dir` settings remain compatible only when the
jobs path is exactly `<assistant_dir>/jobs`. For separate legacy paths, move
`SOUL.md`, context, and jobs under one directory and replace both settings with
`assistant_root`.

### Runtime path compatibility

Explicit `state_path`, `database_path`, `audit_log_path`, and `jobs_run_dir`
settings remain supported for the rest of the 0.x release line. frwrd does not
move or rewrite data at the database, audit, or run-lock paths. On startup,
frwrd imports an existing JSON file at `state_path` into `database_path` once,
records the import in the same SQLite transaction, and leaves the JSON
untouched as a recovery copy. After that commit, frwrd reads and writes cursor
and backend-session state only in SQLite. Do not edit the retained JSON
expecting live state to change.

A future removal of these settings would require an announced major release
and migration instructions. New installations should omit them and let
`FRWRD_HOME` own the whole runtime layout.
