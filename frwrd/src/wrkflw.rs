//! Best-effort mirror of gateway conversations into the wrkflw control plane.
//!
//! When enabled with `[wrkflw] mirror = true` plus a token, the gateway keeps
//! one control-plane inbox task per conversation thread. The first inbound
//! message creates the task; later messages and every assistant reply become
//! task entries. The mirror is fire-and-forget: failures are logged and never
//! block the channel loop.

use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use anyhow::Context;
use reqwest::StatusCode;
use serde::{Deserialize, Serialize};
use tracing::{debug, warn};

use crate::config::Config;

const TASK_TITLE_RUNES: usize = 300;
const TASK_TEXT_BYTES: usize = 16 * 1024;
const IDEMPOTENCY_KEY_BYTES: usize = 200;

#[derive(Debug, Serialize)]
struct CreateTaskInput {
    title: String,
    description: String,
    scheduledDate: String,
    kind: String,
    status: String,
    priority: String,
    overrideLimit: bool,
}

#[derive(Debug, Serialize)]
struct CreateEntryInput {
    kind: String,
    body: String,
}

#[derive(Debug, Deserialize)]
struct TaskResponse {
    id: String,
}

/// Mirrors one conversation thread into the wrkflw control plane.
#[derive(Clone)]
pub struct Wrkflw {
    client: reqwest::Client,
    base_url: String,
    token: String,
    threads: Arc<Mutex<HashMap<String, String>>>,
}

impl Wrkflw {
    /// Builds a mirror from config, or `None` when mirroring is disabled.
    pub fn from_config(cfg: &Config) -> Option<Self> {
        if !cfg.wrkflw_mirror_enabled() {
            return None;
        }
        let Some(token) = cfg.wrkflw_token() else {
            return None;
        };
        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(10))
            .build()
            .ok()?;
        Some(Self {
            client,
            base_url: cfg.wrkflw_base_url().trim_end_matches('/').to_string(),
            token,
            threads: Arc::new(Mutex::new(HashMap::new())),
        })
    }

    /// Mirrors an accepted inbound message into the thread's control-plane
    /// task, creating the task on the thread's first message.
    pub async fn mirror_inbound(&self, channel: &str, event_id: &str, thread: &str, text: &str) {
        let key = thread_key(channel, thread);
        let task_id = self.task_for(&key);
        let Some(task_id) = task_id else {
            if let Err(error) = self.create_thread_task(&key, thread, text, event_id).await {
                warn!("wrkflw mirror: create task for {thread:?} failed: {error:#}");
            }
            return;
        };
        if let Err(error) = self
            .create_entry(
                &task_id,
                text.trim(),
                &format!("frwrd:{channel}:{event_id}:in"),
            )
            .await
        {
            warn!("wrkflw mirror: inbound entry for {thread:?} failed: {error:#}");
        }
    }

    /// Mirrors one delivered reply onto the thread's control-plane task. Does
    /// nothing when the thread has no task yet (for example after a restart).
    pub async fn mirror_reply(&self, channel: &str, thread: &str, inbound_id: i64, text: &str) {
        let key = thread_key(channel, thread);
        let Some(task_id) = self.task_for(&key) else {
            debug!("wrkflw mirror: no task for {thread:?}; reply not mirrored");
            return;
        };
        if let Err(error) = self
            .create_entry(
                &task_id,
                text.trim(),
                &format!("frwrd:{channel}:{inbound_id}:out"),
            )
            .await
        {
            warn!("wrkflw mirror: reply entry for {thread:?} failed: {error:#}");
        }
    }

    async fn create_thread_task(
        &self,
        key: &str,
        thread: &str,
        text: &str,
        event_id: &str,
    ) -> anyhow::Result<String> {
        let trimmed = text.trim();
        let title = first_line(trimmed);
        let description = truncate_bytes(trimmed, TASK_TEXT_BYTES);
        let idempotency_key =
            truncate_bytes(&format!("frwrd:task:{event_id}"), IDEMPOTENCY_KEY_BYTES);
        let task = self
            .create_inbox_task(&title, &description, &idempotency_key)
            .await?;
        self.threads
            .lock()
            .unwrap()
            .entry(key.to_string())
            .or_insert_with(|| task.clone());
        debug!("wrkflw mirror: created task {} for {thread:?}", task);
        Ok(task)
    }

    async fn create_inbox_task(
        &self,
        title: &str,
        description: &str,
        idempotency_key: &str,
    ) -> anyhow::Result<String> {
        let input = CreateTaskInput {
            title: truncate_runes(title, TASK_TITLE_RUNES),
            description: description.to_string(),
            scheduledDate: String::new(),
            kind: "action".to_string(),
            status: String::new(),
            priority: String::new(),
            overrideLimit: false,
        };
        let response = self
            .client
            .post(self.api_url("/api/v1/tasks")?)
            .bearer_auth(&self.token)
            .header("Idempotency-Key", idempotency_key)
            .json(&input)
            .send()
            .await?;
        ensure_success(&response, "create inbox task").await?;
        let task: TaskResponse = response.json().await?;
        Ok(task.id)
    }

    async fn create_entry(
        &self,
        task_id: &str,
        body: &str,
        idempotency_key: &str,
    ) -> anyhow::Result<()> {
        let input = CreateEntryInput {
            kind: "comment".to_string(),
            body: truncate_bytes(body, TASK_TEXT_BYTES),
        };
        let response = self
            .client
            .post(self.api_url(&format!("/api/v1/tasks/{task_id}/entries"))?)
            .bearer_auth(&self.token)
            .header("Idempotency-Key", idempotency_key)
            .json(&input)
            .send()
            .await?;
        ensure_created(&response, "post task entry").await?;
        Ok(())
    }

    fn api_url(&self, path: &str) -> anyhow::Result<String> {
        let base = reqwest::Url::parse(&self.base_url)
            .with_context(|| format!("invalid wrkflw base URL {}", self.base_url))?;
        base.join(path)
            .map(|url| url.to_string())
            .with_context(|| format!("build wrkflw API URL from {path}"))
    }

    fn task_for(&self, key: &str) -> Option<String> {
        self.threads.lock().unwrap().get(key).cloned()
    }
}

fn thread_key(channel: &str, thread: &str) -> String {
    format!("{channel}:{thread}")
}

fn first_line(text: &str) -> String {
    let line = text
        .split('\n')
        .map(str::trim)
        .find(|line| !line.is_empty())
        .unwrap_or(text);
    truncate_runes(line, TASK_TITLE_RUNES)
}

fn truncate_runes(text: &str, limit: usize) -> String {
    if text.chars().count() <= limit {
        return text.to_string();
    }
    text.chars().take(limit).collect()
}

fn truncate_bytes(text: &str, limit: usize) -> String {
    if text.len() <= limit {
        return text.to_string();
    }
    let mut end = limit;
    while !text.is_char_boundary(end) {
        end -= 1;
    }
    text[..end].to_string()
}

async fn ensure_created(response: &reqwest::Response, what: &str) -> anyhow::Result<()> {
    let status = response.status();
    if status.is_success() || status == StatusCode::CONFLICT {
        return Ok(());
    }
    anyhow::bail!("{what} returned HTTP {status}")
}

async fn ensure_success(response: &reqwest::Response, what: &str) -> anyhow::Result<()> {
    let status = response.status();
    if status.is_success() {
        return Ok(());
    }
    anyhow::bail!("{what} returned HTTP {status}")
}
