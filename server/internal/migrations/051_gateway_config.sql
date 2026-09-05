-- Per-account messaging gateway configuration, pulled by the frwrd daemon.
-- Channel secrets are account-owned credentials: readable only through an
-- account-scoped credential, never through an agent credential, and never
-- logged. Hosted storage is opt-in: leaving every field empty disables it
-- and the gateway keeps using its local config file.
CREATE TABLE gateway_configs (
	user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
	channel text NOT NULL DEFAULT '',
	agent text NOT NULL DEFAULT '',
	telegram_bot_token text NOT NULL DEFAULT '',
	telegram_allow_user_ids bigint[] NOT NULL DEFAULT '{}',
	telegram_allow_chat_ids bigint[] NOT NULL DEFAULT '{}',
	slack_app_token text NOT NULL DEFAULT '',
	slack_bot_token text NOT NULL DEFAULT '',
	slack_allow_user_ids text[] NOT NULL DEFAULT '{}',
	imessage_self_handles text[] NOT NULL DEFAULT '{}',
	imessage_allow_from text[] NOT NULL DEFAULT '{}',
	primary_channel text NOT NULL DEFAULT '',
	primary_target text NOT NULL DEFAULT '',
	routes jsonb NOT NULL DEFAULT '[]',
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	last_pulled_at timestamptz
);
