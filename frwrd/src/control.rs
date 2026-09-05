//! Control-plane pulled gateway configuration.
//!
//! When the local config sets `[wrkflw] pull_config = true`, the daemon
//! fetches its channel setup from the wrkflw app instead of requiring every
//! value in the local file. The app is the single control surface; the file
//! on the gateway machine keeps only what the server cannot know
//! (`assistant_root`) plus the token that authorizes the pull itself.
//!
//! Pull failures never stop the gateway: it warns and continues with the
//! local file, so a control-plane outage cannot take messaging down.

use anyhow::{Context, Result};
use serde::Deserialize;

use crate::config::{Config, PrimaryDeliveryConfig, RouteRule};

#[derive(Debug, Deserialize, Default)]
#[serde(default, rename_all = "camelCase")]
struct PulledTelegram {
    bot_token: String,
    allow_user_ids: Vec<i64>,
    allow_chat_ids: Vec<i64>,
}

#[derive(Debug, Deserialize, Default)]
#[serde(default, rename_all = "camelCase")]
struct PulledSlack {
    app_token: String,
    bot_token: String,
    allow_user_ids: Vec<String>,
}

#[derive(Debug, Deserialize, Default)]
#[serde(default, rename_all = "camelCase")]
struct PulledIMessage {
    self_handles: Vec<String>,
    allow_from: Vec<String>,
}

#[derive(Debug, Deserialize, Default)]
#[serde(default, rename_all = "camelCase")]
struct PulledDelivery {
    channel: String,
    target: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PulledRoute {
    thread: String,
    agent: String,
}

#[derive(Debug, Deserialize, Default)]
#[serde(default, rename_all = "camelCase")]
pub(crate) struct PulledConfig {
    channel: String,
    agent: String,
    telegram: PulledTelegram,
    slack: PulledSlack,
    imessage: PulledIMessage,
    primary_delivery: PulledDelivery,
    routes: Vec<PulledRoute>,
}

/// Fetch the account's gateway config. The token authorizes as the account
/// owner; agent credentials are rejected server-side.
pub async fn pull(base_url: &str, token: &str) -> Result<PulledConfig> {
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(15))
        .build()
        .context("build control-plane client")?;
    let url = format!("{}/api/v1/gateway/pull", base_url.trim_end_matches('/'));
    let response = client
        .post(url)
        .bearer_auth(token)
        .send()
        .await
        .context("request gateway config")?;
    let status = response.status();
    if !status.is_success() {
        anyhow::bail!("gateway config pull failed with status {status}");
    }
    response
        .json::<PulledConfig>()
        .await
        .context("decode gateway config")
}

/// Apply a pulled config onto the local one. The server is the source of
/// truth for channels, allowlists, routes, and delivery; local secrets are
/// only replaced when the server actually sends a value, so an empty hosted
/// field can never wipe a working local secret.
pub fn apply(cfg: &mut Config, pulled: PulledConfig) {
    if !pulled.channel.trim().is_empty() {
        cfg.channel = pulled.channel.trim().to_string();
        cfg.channels.clear();
    }
    if !pulled.agent.trim().is_empty() {
        cfg.agent = pulled.agent.trim().to_string();
    }
    if !pulled.telegram.bot_token.trim().is_empty() {
        cfg.telegram_bot_token = Some(pulled.telegram.bot_token.trim().to_string());
    }
    cfg.telegram_allow_user_ids = pulled.telegram.allow_user_ids;
    cfg.telegram_allow_chat_ids = pulled.telegram.allow_chat_ids;
    if !pulled.slack.app_token.trim().is_empty() {
        cfg.slack_app_token = Some(pulled.slack.app_token.trim().to_string());
    }
    if !pulled.slack.bot_token.trim().is_empty() {
        cfg.slack_bot_token = Some(pulled.slack.bot_token.trim().to_string());
    }
    cfg.slack_allow_user_ids = pulled.slack.allow_user_ids;
    cfg.self_handles = pulled.imessage.self_handles;
    cfg.allow_from = pulled.imessage.allow_from;
    if pulled.primary_delivery.channel.trim().is_empty() {
        cfg.primary_delivery = None;
    } else {
        cfg.primary_delivery = Some(PrimaryDeliveryConfig {
            channel: pulled.primary_delivery.channel.trim().to_string(),
            target: pulled.primary_delivery.target.clone(),
        });
    }
    cfg.routes = pulled
        .routes
        .into_iter()
        .map(|route| RouteRule {
            thread: Some(route.thread),
            channel: None,
            agent: route.agent,
        })
        .collect();
}

/// Pull and apply when the local config opts in. Never fails: without
/// opt-in it is a no-op, without a token it warns, and a failed pull warns
/// and keeps the local file so messaging survives a control-plane outage.
pub fn maybe_pull(cfg: &mut Config) {
    if !cfg.wrkflw_pull_config {
        return;
    }
    let Some(token) = cfg.wrkflw_token() else {
        eprintln!("warning: [wrkflw] pull_config is set but no token is configured; using local channel config");
        return;
    };
    let base_url = cfg.wrkflw_base_url();
    match pull_blocking(&base_url, &token) {
        Ok(pulled) => {
            apply(cfg, pulled);
            eprintln!("frwrd: applied channel config pulled from {base_url}");
        }
        Err(error) => {
            eprintln!(
                "warning: control-plane config pull failed ({error:#}); using local channel config"
            );
        }
    }
}

fn pull_blocking(base_url: &str, token: &str) -> Result<PulledConfig> {
    if tokio::runtime::Handle::try_current().is_ok() {
        return tokio::task::block_in_place(|| {
            tokio::runtime::Builder::new_current_thread()
                .enable_all()
                .build()
                .context("build pull runtime")?
                .block_on(pull(base_url, token))
        });
    }
    tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .context("build pull runtime")?
        .block_on(pull(base_url, token))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn pulled(json: &str) -> PulledConfig {
        serde_json::from_str(json).expect("fixture deserializes")
    }

    #[test]
    fn server_payload_deserializes() {
        let cfg = pulled(
            r#"{"channel":"telegram","agent":"codex","telegram":{"botToken":"tok","allowUserIds":[123],"allowChatIds":[]},"slack":{"appToken":"","botToken":"","allowUserIds":[]},"imessage":{"selfHandles":[],"allowFrom":[]},"primaryDelivery":{"channel":"","target":""},"routes":[{"thread":"telegram:dm:123","agent":"claude"}],"updatedAt":"2026-09-05T00:00:00Z"}"#,
        );
        assert_eq!(cfg.channel, "telegram");
        assert_eq!(cfg.telegram.bot_token, "tok");
        assert_eq!(cfg.telegram.allow_user_ids, vec![123]);
        assert_eq!(cfg.routes.len(), 1);
    }

    #[test]
    fn apply_prefers_server_channels_but_keeps_local_secrets() {
        let mut cfg = crate::test_support::test_config();
        cfg.telegram_bot_token = Some("local-secret".to_string());
        cfg.channels = vec!["slack".to_string()];
        let server = pulled(
            r#"{"channel":"telegram","agent":"codex","telegram":{"botToken":"","allowUserIds":[7],"allowChatIds":[]},"slack":{"appToken":"","botToken":"","allowUserIds":[]},"imessage":{"selfHandles":["me"],"allowFrom":[]},"primaryDelivery":{"channel":"telegram","target":"7"},"routes":[]}"#,
        );
        apply(&mut cfg, server);
        assert_eq!(cfg.channel, "telegram");
        assert!(cfg.channels.is_empty());
        assert_eq!(cfg.telegram_bot_token.as_deref(), Some("local-secret"));
        assert_eq!(cfg.telegram_allow_user_ids, vec![7]);
        assert_eq!(cfg.self_handles, vec!["me".to_string()]);
        let delivery = cfg.primary_delivery.expect("delivery set");
        assert_eq!(delivery.channel, "telegram");
        assert_eq!(delivery.target, "7");
    }

    #[test]
    fn apply_replaces_secrets_and_clears_delivery_when_sent() {
        let mut cfg = crate::test_support::test_config();
        cfg.primary_delivery = Some(PrimaryDeliveryConfig {
            channel: "slack".to_string(),
            target: "old".to_string(),
        });
        let server = pulled(
            r#"{"channel":"","agent":"","telegram":{"botToken":"new-secret","allowUserIds":[],"allowChatIds":[]},"slack":{"appToken":"","botToken":"","allowUserIds":[]},"imessage":{"selfHandles":[],"allowFrom":[]},"primaryDelivery":{"channel":"","target":""},"routes":[{"thread":"telegram:dm:9","agent":"pi"}]}"#,
        );
        let before = cfg.channel.clone();
        apply(&mut cfg, server);
        assert_eq!(cfg.channel, before);
        assert_eq!(cfg.telegram_bot_token.as_deref(), Some("new-secret"));
        assert!(cfg.primary_delivery.is_none());
        assert_eq!(cfg.routes.len(), 1);
        assert_eq!(cfg.routes[0].agent, "pi");
    }
}
