# Messaging setup: phone to board in five minutes

The app is the single control surface. The gateway daemon (`frwrd`) runs on
a machine you own and pulls its channel setup from the app, so there is
exactly one screen to learn and one small file on the machine.

## 1. Pair Telegram in the app

1. Message `@BotFather` on Telegram, send `/newbot`, and keep the token.
2. Open **Settings → Messaging** in wrkflw.
3. Under **Channel**, pick Telegram, pick your agent backend, and save.
4. Under **Pair your phone**, type the bot username and scan the QR code
   with your phone. Send the bot one message.
5. Find your numeric Telegram user ID from the Bot API `getUpdates`
   response and add it to the allowed user IDs. Save again.

No BotFather, no QR: iMessage works only on the Mac running the gateway
(grant Full Disk Access first), and Slack connects through Socket Mode
with an installed workspace app. Both are configured on the same screen;
neither supports QR pairing.

## 2. Connect the daemon

On the machine that will run the gateway (Mac or Linux; Windows via WSL):

```sh
WRKFLW_BASE_URL="https://wrkflw" WRKFLW_API_TOKEN="wrkflw_..." \
  sh scripts/setup-gateway.sh
```

The script installs `frwrd` when it is missing, creates the assistant
repository, and writes the minimal local file. `WRKFLW_API_TOKEN` comes
from the Messaging tab (**Create token**) or Settings → API access.
For a local server, point `WRKFLW_BASE_URL` at it
(`http://127.0.0.1:8080`).

Prefer to do it by hand? Install once with
`cargo build --locked --release` inside `frwrd/`, run
`frwrd init ~/Code/assistant`, and write `$FRWRD_HOME/config.toml`:

```toml
assistant_root = "~/Code/assistant"

[wrkflw]
base_url = "https://wrkflw"
token = "wrkflw_..."
pull_config = true
mirror = true
```

That is the whole file. Channels, allowlists, and routes come from the
app on every start; these lines only say where the app is and who may
ask. `pull_config` without a token warns and falls back to the local
file, so messaging survives a control-plane outage.

## 3. Validate and run

```sh
frwrd doctor
frwrd
```

`doctor` checks the pulled channel config, the allowlists, the agent
backend binary, and the mirror. Send a new message after the gateway
starts; the first run deliberately skips the old backlog. The first
accepted message creates an inbox task, and every later message and
reply becomes a task entry. Watch them land on the Inbox page.

Keep it online with `launchd` (macOS) or `systemd` (Linux); never run
two gateways with the same Telegram token, because Telegram allows a
single poller. Rotate a leaked token with BotFather, revoke a leaked
API token in Settings → API access, and rotate the agent credential on
the agent's page. Each secret has exactly one place to die.
